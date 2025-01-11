package slice_on_disk

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func intSlicer() Slicer[int] {
	slice := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	sl, _ := New(slice, os.TempDir())
	for i := 10; i < 100; i++ {
		sl.Append(i)
	}
	return sl
}

func TestSlice(t *testing.T) {

	tests := []struct {
		name  string
		slice []string
	}{
		{
			name: "append and left shift",
			slice: []string{
				strings.Repeat("0", rand.Intn(200000)),
				strings.Repeat("1", rand.Intn(200000)),
				strings.Repeat("2", rand.Intn(200000)),
				strings.Repeat("3", rand.Intn(200000)),
				strings.Repeat("4", rand.Intn(200000)),
			},
		},

		{
			name: "slice, get, put",
		},
	}

	//for _, tt := range tests {

	//}
	tt := tests[0]
	t.Run(tt.name, func(t *testing.T) {

		t.Log(tt.name)
		s, err := New(tt.slice, os.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer s.Cleanup()

		err = s.Delete(0, 3)
		if s.Len() != 2 {
			t.Errorf("Len() = %d, want 2", s.Len())
		}

		start := time.Now()
		var total int64
		for i := 100; i < 10003; i++ {
			char := strconv.Itoa(i)
			l := rand.Intn(10000)
			total += int64(l)
			s.Append(strings.Repeat(char, l))
		}

		t.Logf("9903 total length %d took %v", total, time.Since(start))

		c, _ := s.(*config[string])
		if cap(c.slice) != 5 || len(c.slice) != 5 || len(c.diskSlice) != 9900 {
			t.Errorf("unexpeted len or cap: cap=%d, len=%d, disklen=%d", cap(c.slice), len(c.slice), len(c.diskSlice))
		}

		s.Delete(0, 3)
		if cap(c.slice) != 5 || len(c.slice) != 5 || len(c.diskSlice) != 9897 {
			t.Errorf("unexpeted len or cap: cap=%d, len=%d, disklen=%d", cap(c.slice), len(c.slice), len(c.diskSlice))
		}

		s.Delete(0, 100)
		if cap(c.slice) != 5 || len(c.slice) != 5 || len(c.diskSlice) != 9797 || s.Len() != 9797+5 {
			t.Errorf("unexpeted len or cap: cap=%d, len=%d, disklen=%d, c.Len()=%d", cap(c.slice), len(c.slice), len(c.diskSlice), c.Len())
		}
		s.Cleanup()

		sl := intSlicer()
		sl.Delete(7, 5)
		// 7,8,9 from slice, 10,11 from diskslice are gone
		// 12,13,14 from diskslice replace 7,8,9
		cl, _ := sl.(*config[int])
		if cap(cl.slice) != 10 || len(cl.slice) != 10 || len(cl.diskSlice) != 85 || sl.Len() != 85+10 {
			t.Errorf("unexpeted len or cap: cap=%d, len=%d, disklen=%d, c.Len()=%d", cap(cl.slice), len(cl.slice), len(cl.diskSlice), sl.Len())
		}
		if x, _ := sl.Get(7); x != 12 {
			t.Errorf("unexpeted value of sl[7]: x=%d, want 12", x)
		}
		sl.Cleanup()

		sl = intSlicer()
		sl.Delete(20, 7)
		cl, _ = sl.(*config[int])

		if cap(cl.slice) != 10 || len(cl.slice) != 10 || len(cl.diskSlice) != 83 || sl.Len() != 83+10 {
			t.Errorf("unexpeted len or cap: cap=%d, len=%d, disklen=%d, c.Len()=%d", cap(cl.slice), len(cl.slice), len(cl.diskSlice), sl.Len())
		}
		if x, _ := sl.Get(20); x != 27 {
			t.Errorf("unexpeted value of sl[20]: x=%d, want 27", x)
		}
		sl.Cleanup()

	})

	tt = tests[1]
	t.Run(tt.name, func(t *testing.T) {
		slice := make([]int, 5, 5)
		for i := range slice {
			slice[i] = i
		}

		s, err := New(slice, os.TempDir())
		if err != nil {
			t.Fatal(err)
		}

		for i := 5; i < 100; i++ {
			err = s.Append(i)
			if err != nil {
				t.Fatal(err)
			}
		}

		x, err := s.Slice(3, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(x) != 7 {
			t.Errorf("len %d, want 7", len(x))
		}
		for i := 3; i < 10; i++ {
			if x[i-3] != i {
				t.Errorf("%d, want %d", x[i-3], i)
			}
		}

		a, _ := s.Get(0)
		b, _ := s.Get(7)
		if a != 0 || b != 7 {
			t.Errorf("get failed %d %d, want 0, 7", a, b)
		}

		s.Put(0, 199)
		s.Put(7, 99)
		a, _ = s.Get(0)
		b, _ = s.Get(7)
		if a != 199 || b != 99 {
			t.Errorf("get failed %d %d, want 0, 7", a, b)
		}
		s.Cleanup()
	})

}

type msg struct {
	placed  time.Time
	payload string
}

func TestLong(t *testing.T) {
	b, err := os.ReadFile("./test/long.txt")
	if err != nil {
		fmt.Print(err)
	}

	m := msg{time.Now(), string(b)}
	overflow, err := New(make([]msg, 0, 512), os.TempDir())
	if err != nil {
		t.Errorf("error creating overflow: %s", err)
	}
	err = overflow.Append(m)
	if err != nil {
		t.Errorf("error appending: %s", err)
	}
	m, err = overflow.Get(overflow.Len() - 1)
	if err != nil {
		t.Errorf("error getting overflow %d: %s", overflow.Len()-1, err)
	}
	t.Logf("overflow len %d, payload len: %d", overflow.Len(), len(m.payload))

}
