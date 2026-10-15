// Command seed fills a running local server with a week of sessions, through
// its own API, so screens can be tried on real data. It signs in as
// dev@passion.local, or signs that account up first.
//
//	go run ./server/cmd/seed [-addr http://localhost:8080] [-reset]
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"passion/server/db"
)

const (
	email    = "dev@passion.local"
	password = "passion-dev"
)

type set struct {
	Reps       *int     `json:"reps"`
	Seconds    *int     `json:"seconds"`
	RepSeconds []int    `json:"rep_seconds"`
	WeightKG   *float64 `json:"weight_kg"`
}

type climb struct {
	Discipline  string   `json:"discipline"`
	Setting     string   `json:"setting"`
	Board       *string  `json:"board"`
	RopeStyle   *string  `json:"rope_style"`
	Grade       *string  `json:"grade"`
	GradeSystem *string  `json:"grade_system"`
	Goes        []string `json:"goes"`
	Seconds     *int     `json:"seconds"`
	Focus       *string  `json:"focus"`
	Notes       *string  `json:"notes"`
}

// A step with nothing logged is skipped when its run finishes.
type step struct {
	section  string
	exercise string
	notes    string
	sets     []set
	climbs   []climb
}

type journal struct {
	Sleep     *int    `json:"sleep"`
	Energy    *int    `json:"energy"`
	RPE       *int    `json:"rpe"`
	Focus     *string `json:"focus"`
	Setting   *string `json:"setting"`
	WentWell  *string `json:"went_well"`
	NextFocus *string `json:"next_focus"`
}

type day struct {
	ago     int
	name    string
	minutes int
	place   string
	journal journal
	open    bool
	steps   []step
}

func ptr[T any](v T) *T { return &v }

func reps(n int, kg float64) set { return set{Reps: &n, WeightKG: &kg} }
func hold(s int) set             { return set{Seconds: &s} }
func hang(s int, kg float64) set { return set{Seconds: &s, WeightKG: &kg} }
func repeat(n, s int) set        { return set{Reps: &n, Seconds: &s} }
func ladder(kg float64, s ...int) set {
	return set{RepSeconds: s, WeightKG: &kg}
}

func route(grade string, style string, goes ...string) climb {
	return climb{Discipline: "sport", Setting: "outdoor", RopeStyle: &style, Grade: &grade, GradeSystem: ptr("french"), Goes: goes}
}

func boulder(grade string, goes ...string) climb {
	return climb{Discipline: "boulder", Setting: "indoor", Grade: &grade, GradeSystem: ptr("font"), Goes: goes}
}

func traverse(seconds int, notes string) climb {
	c := climb{Discipline: "boulder", Setting: "indoor", Grade: ptr("Traverse"), Goes: []string{"attempt"}, Seconds: &seconds}
	if notes != "" {
		c.Notes = &notes
	}
	return c
}

func (c climb) on(board string) climb  { c.Board = &board; return c }
func (c climb) note(text string) climb { c.Notes = &text; return c }
func (c climb) aim(text string) climb  { c.Focus = &text; return c }

var week = []day{
	{
		ago: 12, name: "Crag day", minutes: 310, place: "Sector Cave",
		journal: journal{Sleep: ptr(5), Energy: ptr(4), RPE: ptr(8), Focus: ptr("projects"), Setting: ptr("outdoor"),
			WentWell:  ptr("Finally linked the crux into the top section. Clipping from the good jug instead of the crimp saved a lot."),
			NextFocus: ptr("Warm up on two routes, not one. The first go was wasted on cold fingers.")},
		steps: []step{
			{section: "Warmup", exercise: "Easy Lead Mileage", climbs: []climb{
				route("6a", "lead", "send"),
				route("6b", "lead", "send").note("Polished feet low down, trust them anyway."),
			}},
			{section: "Main", exercise: "Redpoint Lead Attempts", notes: "Conditions were good until about three, then the sun hit the wall.",
				climbs: []climb{
					route("7a", "lead", "attempt", "attempt", "send").aim("Breathe at the rest before the roof").
						note("First go: fell at the undercling, pumped. Second go: made the crux but missed the clip.\n\nThird go felt smooth. Shaking out longer at the kneebar made the difference."),
					route("7a+", "lead", "attempt").note("Bolt-to-bolt to work the moves. The start is fine, the middle needs a better left foot."),
				}},
			{section: "Cooldown", exercise: "Frog Stretch", sets: []set{hold(60), hold(60)}},
		},
	},
	{
		ago: 8, name: "Fingers and pulls", minutes: 70,
		journal: journal{Sleep: ptr(4), Energy: ptr(3), RPE: ptr(6), Focus: ptr("strength"), Setting: ptr("indoor")},
		steps: []step{
			{section: "Warmup", exercise: "Pulse Raiser", sets: []set{hold(420)}},
			{section: "Hangboard", exercise: "Repeaters 7:3 (20 mm Edge)", sets: []set{repeat(6, 7), repeat(6, 7), repeat(5, 7)}},
			{section: "Hangboard", exercise: "Isometric Hang", notes: "Left ring finger a bit tender on the last ladder. Keep an eye on it.",
				sets: []set{ladder(5, 3, 6, 9), ladder(5, 3, 6, 9), ladder(7.5, 3, 6)}},
			{section: "Pulling", exercise: "Weighted Pull-ups", sets: []set{reps(6, -10), reps(6, -10), reps(5, 0)}},
			{section: "Pulling", exercise: "Bar/Ring Rows", sets: []set{reps(12, 0), reps(12, 0), reps(10, 0)}},
			{section: "Core", exercise: "Copenhagen Adduction", sets: []set{reps(8, 0), reps(8, 0)}},
			{section: "Cooldown", exercise: "Pigeon Pose", sets: []set{hold(90), hold(90)}},
		},
	},
	{
		ago: 6, name: "Open session", minutes: 123, place: "Home wall",
		journal: journal{Sleep: ptr(3), Energy: ptr(3), RPE: ptr(4), Focus: ptr("endurance"), Setting: ptr("indoor"),
			WentWell: ptr("Feet stayed quiet on the slab."), NextFocus: ptr("Start the traverses fresher.")},
		steps: []step{
			{section: "Warmup", exercise: "General Warmup", sets: []set{hold(382)}},
			{section: "Warmup", exercise: "Silent Feet", climbs: []climb{
				boulder("5c", "send").aim("Place each foot once"),
				boulder("6a", "attempt", "send").note("Scuffed the volume on the first go"),
				{Discipline: "boulder", Setting: "indoor", Goes: []string{"send"}},
			}},
			{section: "Drills", exercise: "Hover Hands", climbs: []climb{
				{Discipline: "boulder", Setting: "indoor", Grade: ptr("Rainbow"), Goes: []string{"attempt", "attempt"}},
			}},
			{section: "Endurance", exercise: "Traverse Circuit", climbs: []climb{
				traverse(180, "Pumped at the corner"),
				traverse(420, ""),
				traverse(420, "Shook out on the big jug, felt better"),
			}},
			{section: "Cooldown", exercise: "Pigeon Pose"},
		},
	},
	{
		ago: 4, name: "Strength", minutes: 75,
		journal: journal{Sleep: ptr(4), Energy: ptr(4), RPE: ptr(7), Focus: ptr("strength"), Setting: ptr("indoor")},
		steps: []step{
			{section: "Warmup", exercise: "Joint Circles", sets: []set{hold(300)}},
			{section: "Main", exercise: "Weighted Pull-ups", sets: []set{reps(5, 10), reps(5, 12.5), reps(4, 12.5)}},
			{section: "Main", exercise: "Deadlift", notes: "Lower back tight on the last set.",
				sets: []set{reps(5, 80), reps(5, 80), reps(4, 80)}},
			{section: "Main", exercise: "Bench Press", sets: []set{reps(8, 50), reps(8, 50), reps(7, 50)}},
			{section: "Auxiliary", exercise: "Face Pulls", sets: []set{reps(15, 10), reps(15, 10)}},
			{section: "Auxiliary", exercise: "Lateral Raise"},
		},
	},
	{
		ago: 2, name: "Board and hangs", minutes: 95, place: "Home wall",
		journal: journal{Sleep: ptr(2), Energy: ptr(3), RPE: ptr(8), Focus: ptr("projects"), Setting: ptr("indoor")},
		steps: []step{
			{section: "Warmup", exercise: "General Warmup", sets: []set{hold(480)}},
			{section: "Hangboard", exercise: "Max Hangs (20 mm Edge)",
				sets: []set{hang(10, 15), hang(10, 15), hang(10, 17.5), hang(10, 17.5), hang(8, 17.5)}},
			{section: "Main", exercise: "Boulder Projects", climbs: []climb{
				boulder("6b+", "send").on("moon"),
				boulder("6c", "attempt", "attempt", "send").on("moon").aim("Drive through the hips"),
				boulder("7a", "attempt", "attempt", "attempt", "attempt").on("moon").note("Cannot stick the left crimp yet"),
				boulder("6c+", "send", "send").on("moon"),
			}},
			{section: "Endurance", exercise: "Mock Lead (Auto-Belay)", climbs: []climb{
				{Discipline: "sport", Setting: "indoor", RopeStyle: ptr("auto_belay"), Grade: ptr("6b"), GradeSystem: ptr("french"),
					Goes: []string{"send"}, Seconds: ptr(240)},
				{Discipline: "sport", Setting: "indoor", RopeStyle: ptr("auto_belay"), Grade: ptr("6a+"), GradeSystem: ptr("french"),
					Goes: []string{"send"}, Seconds: ptr(205), Notes: ptr("Clipped every bolt from a straight arm")},
			}},
		},
	},
	{
		ago: 0, name: "Open session", open: true,
		steps: []step{
			{section: "Warmup", exercise: "General Warmup", sets: []set{hold(420)}},
			{section: "Drills", exercise: "Silent Feet", climbs: []climb{boulder("5c", "attempt")}},
			{section: "Drills", exercise: "Traverse Circuit"},
		},
	},
}

func main() {
	addr := flag.String("addr", "http://localhost:8080", "the server to fill")
	reset := flag.Bool("reset", false, "delete the account's runs first")
	flag.Parse()

	if err := run(&client{addr: *addr}, *reset); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(c *client, reset bool) error {
	if err := c.signIn(); err != nil {
		return err
	}

	var runs struct {
		Runs []struct {
			ID string `json:"id"`
		} `json:"runs"`
	}
	if err := c.do("GET", "/api/v1/runs", nil, &runs); err != nil {
		return err
	}
	if len(runs.Runs) > 0 && !reset {
		fmt.Printf("%s already has %d runs. Pass -reset to delete them and seed again.\n", email, len(runs.Runs))
		return nil
	}
	for _, r := range runs.Runs {
		if err := c.do("DELETE", "/api/v1/runs/"+r.ID, nil, nil); err != nil {
			return err
		}
	}

	var library struct {
		Exercises []struct {
			ID string `json:"id"`
			exercise
		} `json:"exercises"`
	}
	if err := c.do("GET", "/api/v1/exercises", nil, &library); err != nil {
		return err
	}
	byName := map[string]stepBody{}
	for _, e := range library.Exercises {
		byName[e.Name] = stepBody{Exercise: e.ID, exercise: e.exercise}
	}

	for _, d := range week {
		if err := c.seed(d, byName); err != nil {
			return fmt.Errorf("%s, %d days ago: %w", d.name, d.ago, err)
		}
	}
	fmt.Printf("Seeded %d runs. Sign in as %s with password %s.\n", len(week), email, password)
	return nil
}

// exercise is the copy a run's step keeps, field for field as the API sends
// an exercise, less what only a library entry has.
type exercise struct {
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	Notes           *string           `json:"notes"`
	Source          *string           `json:"source"`
	Tags            []string          `json:"tags"`
	Sets            *int              `json:"sets"`
	Reps            *int              `json:"reps"`
	SetRestSeconds  *int              `json:"set_rest_seconds"`
	RepSeconds      *int              `json:"rep_seconds"`
	RepRestSeconds  *int              `json:"rep_rest_seconds"`
	PrepSeconds     *int              `json:"prep_seconds"`
	DurationSeconds *int              `json:"duration_seconds"`
	PerSide         bool              `json:"per_side"`
	Ladder          []int             `json:"ladder"`
	Media           []json.RawMessage `json:"media"`
}

type stepBody struct {
	ID       string  `json:"id"`
	Exercise string  `json:"exercise"`
	RunNotes *string `json:"run_notes"`
	exercise
}

type item struct {
	Step stepBody `json:"step"`
}

type section struct {
	Name  string `json:"name"`
	Items []item `json:"items"`
}

func (c *client) seed(d day, library map[string]stepBody) error {
	date := time.Now().AddDate(0, 0, -d.ago)
	started := time.Date(date.Year(), date.Month(), date.Day(), 17, 30, 0, 0, time.Local)
	localDate := started.Format(time.DateOnly)

	var created struct {
		ID string `json:"id"`
	}
	if err := c.do("POST", "/api/v1/runs", map[string]any{"name": d.name, "started_at": started, "local_date": localDate}, &created); err != nil {
		return err
	}

	var sections []section
	ids := make([]string, len(d.steps))
	for i, s := range d.steps {
		body, ok := library[s.exercise]
		if !ok {
			return fmt.Errorf("no exercise named %q", s.exercise)
		}
		body.ID = db.NewID()
		ids[i] = body.ID
		if s.notes != "" {
			body.RunNotes = &s.notes
		}
		if len(sections) == 0 || sections[len(sections)-1].Name != s.section {
			sections = append(sections, section{Name: s.section})
		}
		last := &sections[len(sections)-1]
		last.Items = append(last.Items, item{body})
	}

	update := struct {
		Name           string    `json:"name"`
		Sections       []section `json:"sections"`
		StartedAt      time.Time `json:"started_at"`
		LocalDate      string    `json:"local_date"`
		ElapsedSeconds *int      `json:"elapsed_seconds"`
		Place          *string   `json:"place"`
		journal
	}{Name: d.name, Sections: sections, StartedAt: started, LocalDate: localDate, journal: d.journal}
	if d.minutes > 0 {
		update.ElapsedSeconds = ptr(d.minutes * 60)
	}
	if d.place != "" {
		update.Place = &d.place
	}
	if err := c.do("PUT", "/api/v1/runs/"+created.ID, update, nil); err != nil {
		return err
	}

	position := 0
	for i, s := range d.steps {
		if len(s.sets) > 0 {
			if err := c.do("PUT", "/api/v1/runs/"+created.ID+"/steps/"+ids[i]+"/sets", map[string]any{"sets": s.sets}, nil); err != nil {
				return err
			}
		}
		for _, cl := range s.climbs {
			body := struct {
				Step     string `json:"step"`
				Position int    `json:"position"`
				climb
			}{ids[i], position, cl}
			if err := c.do("PUT", "/api/v1/runs/"+created.ID+"/climbs/"+db.NewID(), body, nil); err != nil {
				return err
			}
			position++
		}
	}

	if d.open {
		return nil
	}
	return c.do("POST", "/api/v1/runs/"+created.ID+"/finish", nil, nil)
}

type client struct {
	addr  string
	token string
}

var errUnauthorized = errors.New("unauthorized")

// signIn signs up the dev account the first time it is turned away.
func (c *client) signIn() error {
	var signedIn struct {
		Token string `json:"token"`
	}
	err := c.do("POST", "/api/v1/tokens", map[string]string{"email": email, "password": password}, &signedIn)
	if err == nil {
		c.token = signedIn.Token
		return nil
	}
	if !errors.Is(err, errUnauthorized) {
		return err
	}

	var signedUp struct {
		Token struct {
			Token string `json:"token"`
		} `json:"token"`
	}
	account := map[string]string{"email": email, "password": password, "display_name": "Dev", "timezone": "Europe/Oslo"}
	if err := c.do("POST", "/api/v1/accounts", account, &signedUp); err != nil {
		return err
	}
	c.token = signedUp.Token.Token
	return nil
}

func (c *client) do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.addr+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s %s: %s: %s", method, path, res.Status, msg)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}
