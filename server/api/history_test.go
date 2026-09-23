package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestExerciseHistory(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	run, hang, climb := runWithSteps(t, h, ada)
	const hangExercise = "0199c3a0-0000-7000-8000-0000000000b1"

	send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID+"/steps/"+hang+"/sets", ada, `{"sets": [{"seconds": 10}]}`)
	send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID+"/climbs/0199c3a0-0000-7000-8000-0000000000c1", ada,
		`{"step": "`+climb+`", "discipline": "boulder", "setting": "indoor"}`)

	history := func(who, exercise string) exerciseHistoryResponse {
		t.Helper()
		rec := send(t, h, http.MethodGet, "/api/v1/exercises/"+exercise+"/history", who, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var got exerciseHistoryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	if got := history(ada, hangExercise); len(got.Sessions) != 0 {
		t.Fatalf("sessions %+v before the finish, want none", got.Sessions)
	}
	send(t, h, http.MethodPost, "/api/v1/runs/"+run.ID+"/finish", ada, "")
	got := history(ada, hangExercise)
	if len(got.Sessions) != 1 || got.Sessions[0].Run != run.ID || len(got.Sessions[0].Sets) != 1 || got.Sessions[0].Climbs == nil {
		t.Fatalf("sessions %+v, want the finished run with its one set and an empty climb list", got.Sessions)
	}
	if got := history(bob, hangExercise); len(got.Sessions) != 0 {
		t.Fatalf("bob sees %+v, want nothing of ada's", got.Sessions)
	}
	if rec := send(t, h, http.MethodGet, "/api/v1/exercises/nope/history", ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d for an id that is not a uuid, want 404", rec.Code)
	}
}
