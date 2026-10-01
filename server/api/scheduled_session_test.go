package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func listSchedule(t *testing.T, h http.Handler, bearer, from, to string) []scheduledDayBody {
	t.Helper()
	rec := send(t, h, http.MethodGet, "/api/v1/scheduled-sessions?from="+from+"&to="+to, bearer, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d: %s", rec.Code, rec.Body)
	}
	var got scheduledDayListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got.Days
}

func TestScheduledSessions(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	power := insertShippedSessionTemplate(t, pool, "Power")
	if rec := send(t, h, http.MethodPut, "/api/v1/cycles/"+cycleID, ada, cycleBody(power)); rec.Code != http.StatusOK {
		t.Fatalf("cycle: status %d: %s", rec.Code, rec.Body)
	}

	one := `{"template": "` + power + `", "local_date": "2100-01-06"}`
	rec := send(t, h, http.MethodPost, "/api/v1/scheduled-sessions", ada, one)
	if rec.Code != http.StatusCreated {
		t.Fatalf("schedule: status %d: %s", rec.Code, rec.Body)
	}
	var oneOff scheduledSessionBody
	if err := json.Unmarshal(rec.Body.Bytes(), &oneOff); err != nil {
		t.Fatal(err)
	}
	if oneOff.Cycle != nil || oneOff.LocalDate != "2100-01-06" {
		t.Fatalf("scheduled %+v, want a one-off on the 6th", oneOff)
	}
	if rec := send(t, h, http.MethodPost, "/api/v1/scheduled-sessions", ada, one); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("the same session twice on a day: status %d, want 422", rec.Code)
	}

	if _, err := pool.Exec(t.Context(), `UPDATE session_template SET icon = 'hand' WHERE id = $1`, power); err != nil {
		t.Fatal(err)
	}
	days := listSchedule(t, h, ada, "2100-01-05", "2100-01-07")
	var got []string
	for _, d := range days {
		got = append(got, d.LocalDate+" "+d.TemplateName+" "+d.Status)
		if d.TemplateIcon == nil || *d.TemplateIcon != "hand" {
			t.Fatalf("icon %v, want the template's", d.TemplateIcon)
		}
	}
	want := []string{"2100-01-05 Power planned", "2100-01-06 Power planned", "2100-01-07 Power planned"}
	if !slices.Equal(got, want) {
		t.Fatalf("days %v, want %v", got, want)
	}
	if days[0].Cycle == nil || *days[0].Cycle != cycleID {
		t.Fatalf("cycle %v, want the cycle that placed it", days[0].Cycle)
	}

	// Running it marks the day done.
	run := startRunFor(t, h, ada, `{"scheduled": "`+days[0].ID+`"}`)
	if run.Scheduled == nil || *run.Scheduled != days[0].ID {
		t.Fatalf("run points at %v, want %s", run.Scheduled, days[0].ID)
	}
	send(t, h, http.MethodPost, "/api/v1/runs/"+run.ID+"/finish", ada, "")
	if days := listSchedule(t, h, ada, "2100-01-05", "2100-01-05"); days[0].Status != "done" || *days[0].Run != run.ID {
		t.Fatalf("day %+v, want done by the run", days[0])
	}

	path := "/api/v1/scheduled-sessions/" + oneOff.ID
	if rec := send(t, h, http.MethodPut, path, ada, `{"local_date": "2100-01-07"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("moved onto a day that holds it: status %d, want 422", rec.Code)
	}
	if rec := send(t, h, http.MethodPut, path, bob, `{"local_date": "2100-01-08"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else moved it: status %d, want 404", rec.Code)
	}
	if rec := send(t, h, http.MethodPut, path, ada, `{"local_date": "2100-01-08"}`); rec.Code != http.StatusOK {
		t.Fatalf("move: status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d: %s", rec.Code, rec.Body)
	}
	if days := listSchedule(t, h, ada, "2100-01-08", "2100-01-08"); len(days) != 0 {
		t.Fatalf("days %+v after the delete, want none", days)
	}
	if days := listSchedule(t, h, bob, "2100-01-01", "2100-01-31"); len(days) != 0 {
		t.Fatalf("bob sees %+v, want nothing of ada's", days)
	}

	for name, query := range map[string]string{
		"no range":         "",
		"a bad day":        "?from=soon&to=2100-01-05",
		"a range reversed": "?from=2100-01-05&to=2100-01-04",
	} {
		t.Run(name, func(t *testing.T) {
			if rec := send(t, h, http.MethodGet, "/api/v1/scheduled-sessions"+query, ada, ""); rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", rec.Code)
			}
		})
	}
}

func TestScheduledSessionsNeedSignIn(t *testing.T) {
	h := newTestServer(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/scheduled-sessions"},
		{http.MethodPost, "/api/v1/scheduled-sessions"},
		{http.MethodPut, "/api/v1/scheduled-sessions/" + cycleID},
		{http.MethodDelete, "/api/v1/scheduled-sessions/" + cycleID},
	} {
		if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401", route.method, route.path, rec.Code)
		}
	}
}
