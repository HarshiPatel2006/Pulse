//go:build linux

package linuxmon

import (
	"os"
	"testing"
	"time"
)

func TestPmonColumnNames(t *testing.T) {
	for _, sample := range []string{
		"# gpu pid type sm mem enc dec fb command\n0 42 C 71 5 - - 256 worker\n",
		"# gpu pid type sm mem enc dec jpg ofa fb ccpm command\n0 42 C 71 5 - - - - 256 0 worker\n",
	} {
		v := map[int32]uint64{}
		u := map[int32]float64{}
		parsePmon(sample, v, u)
		if v[42] != 256*1048576 || u[42] != 71 {
			t.Fatalf("incorrect metrics: %v %v", v, u)
		}
	}
	v := map[int32]uint64{}
	u := map[int32]float64{}
	parsePmon("# gpu pid type sm mem command\n0 42 C - - worker", v, u)
	if len(v) != 0 || len(u) != 0 {
		t.Fatal("unsupported values should remain unavailable")
	}
}
func TestCollectorLiveAndHistory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c := New()
	s := c.Collect()
	if s.Total == 0 || len(s.CPU) == 0 || len(s.Procs) == 0 {
		t.Fatalf("missing live data: %+v", s.Errors)
	}
	found := false
	for _, p := range s.Procs {
		if p.PID == int32(os.Getpid()) {
			found = true
			if p.Memory == 0 || p.Start == 0 {
				t.Fatalf("invalid current process: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("current process absent")
	}
	c.History["fixture"] = History{CPU: 2, Read: 10, Write: 20}
	if e := c.Save(); e != nil {
		t.Fatal(e)
	}
	other := New()
	if other.History["fixture"].Read != 10 {
		t.Fatal("history did not survive reload")
	}
	time.Sleep(100 * time.Millisecond)
	second := c.Collect()
	c.History["fixture"] = History{CPU: 99}
	if second.History["fixture"].CPU == 99 {
		t.Fatal("snapshot history aliases mutable collector")
	}
}
