package rislive

import "testing"

func TestMatchPrefix(t *testing.T) {
	// Example/test announcements.
	p4 := &RisAnnouncement{
		NextHop:  "1.2.3.4",
		Prefixes: []string{"192.168.0.0/16", "10.0.0.0/24"},
	}
	p6 := &RisAnnouncement{
		NextHop:  "2001:db8:123::1",
		Prefixes: []string{"2001:db8::/32", "2001:db8:48::/48"},
	}

	tests := []struct {
		desc       string
		ann        *RisAnnouncement
		candidates []string
		want       bool
	}{{
		desc:       "Success v4",
		ann:        p4,
		candidates: []string{"192.168.0.0/16", "100.64.0.0/10"},
		want:       true,
	}, {
		desc:       "Success v6",
		ann:        p6,
		candidates: []string{"2001:db8:32::/32", "2001:db8:48::/48"},
		want:       true,
	}, {
		desc:       "Success v4 match in mixed family",
		ann:        p6,
		candidates: []string{"192.169.0.0/16", "2001:db8:48::/48"},
		want:       true,
	}, {
		desc:       "Success v6 match in mixed family",
		ann:        p6,
		candidates: []string{"2001:db8::/32", "192.169.0.0/16"},
		want:       true,
	}, {
		desc:       "Failure v4",
		ann:        p4,
		candidates: []string{"197.168.0.0/16", "10.64.0.0/10"},
		want:       false,
	}, {
		desc:       "Failure v6",
		ann:        p6,
		candidates: []string{"2001:db8:32::/32", "2001:db9:48::/48"},
		want:       false,
	}, {
		desc:       "Failure v4 with v6 mach",
		candidates: []string{"2001:db8:32::/32", "2001:db8:48::/48"},
		ann:        p4,
		want:       false,
	}, {
		desc:       "Failure v6 with v4 match",
		ann:        p6,
		candidates: []string{"192.168.0.0/16", "100.64.0.0/10"},
		want:       false,
	}}

	for _, test := range tests {
		got := test.ann.MatchPrefix(test.candidates)
		if got != test.want {
			t.Errorf("[%v]: got/want mismatch, got(%v) / want(%v)", test.desc, got, test.want)
		}
	}
}

func TestMatchASPath(t *testing.T) {
	msg01 := &RisMessageData{Path: []int32{1, 2, 3, 4, 5, 6, 7, 8}}
	msg02 := &RisMessageData{Path: []int32{1}}
	msg03 := &RisMessageData{Path: []int32{1, 3, 4, 5, 6, 7, 8}}
	msg04 := &RisMessageData{Path: []int32{1, 3, 2, 4, 5, 6, 7, 8}}

	tests := []struct {
		desc       string
		msg        *RisMessageData
		candidates []int32
		want       bool
	}{{
		desc:       "Success find len(1) path",
		msg:        msg01,
		candidates: []int32{3},
		want:       true,
	}, {
		desc:       "Fail can not find len(1) path",
		msg:        msg01,
		candidates: []int32{10},
		want:       false,
	}, {
		desc:       "Success can find len(2) path",
		msg:        msg01,
		candidates: []int32{3, 4},
		want:       true,
	}, {
		desc:       "Success can find len(3) path",
		msg:        msg01,
		candidates: []int32{3, 4, 5},
		want:       true,
	}, {
		desc:       "Success candidate path too long",
		msg:        msg02,
		candidates: []int32{3, 4, 5},
		want:       false,
	}, {
		desc:       "Success candidate path not in mesg",
		msg:        msg03,
		candidates: []int32{2, 3, 4},
		want:       false,
	}, {
		desc:       "Success candidate path in wrong order from mesg",
		msg:        msg04,
		candidates: []int32{2, 3, 4},
		want:       false,
	}}

	for _, test := range tests {
		got := test.msg.MatchASPath(test.candidates)
		if got != test.want {
			t.Errorf("[%v]: got/want mismatch, got(%v) / want(%v)", test.desc, got, test.want)
		}
	}
}
