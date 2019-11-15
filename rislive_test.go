package main

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
)

var (
	msg01 = &RisMessageData{Path: []int32{1, 2, 3, 4, 5, 6, 7, 8}, Origin: "8"}
	msg02 = &RisMessageData{Path: []int32{1}, Origin: "1"}
	msg03 = &RisMessageData{Path: []int32{1, 3, 4, 5, 6, 7, 8}, Origin: "8"}
	msg04 = &RisMessageData{Path: []int32{1, 3, 2, 4, 5, 6, 7, 8}, Origin: "8"}
)

func TestNewRisFilter(t *testing.T) {
	tests := []struct {
		desc            string
		aspath          []int32
		transits        map[int32]bool
		origins, prefix []string
		want            *RisFilter
	}{{
		desc:     "Success NewRisFilter",
		aspath:   []int32{1, 2, 3},
		transits: map[int32]bool{1: true, 2: true},
		origins:  []string{"1", "2"},
		prefix:   []string{"192.168.1.0/24", "10.1.0.0/16"},
		want: &RisFilter{
			ASPath:           []int32{1, 2, 3},
			InvalidTransitAS: map[int32]bool{1: true, 2: true},
			Origins:          []string{"1", "2"},
			Prefix:           []string{"192.168.1.0/24", "10.1.0.0/16"},
		},
	}}

	for _, test := range tests {
		got := NewRisFilter(test.aspath, test.transits, test.origins, test.prefix)
		if !cmp.Equal(got, test.want) {
			t.Errorf("[%v]: got/want mismatch diff(-got, +want):\n%v\n", test.desc, cmp.Diff(got, test.want))
		}
	}
}

func TestNewRisLive(t *testing.T) {
	tests := []struct {
		desc    string
		url, ua string
		file    *string
		rf      RisFilter
		buffer  int
		want    *RisLive
	}{{
		desc:   "Success - nil file",
		url:    "http://blah",
		file:   nil,
		ua:     "foo",
		rf:     RisFilter{ASPath: []int32{1}},
		buffer: 10,
		want: &RisLive{
			URL:    proto.String("http://blah"),
			UA:     proto.String("foo"),
			Filter: &RisFilter{ASPath: []int32{1}},
			Chan:   make(chan (RisMessage), 10),
		},
	}}

	for _, test := range tests {
		got := NewRisLive(&test.url, test.file, &test.ua, &test.rf, &test.buffer)
		if !cmp.Equal(got.URL, test.want.URL) && !cmp.Equal(got.UA, test.want.UA) {
			t.Errorf("[%v]: got/want mismatch, diff (-got, +want):\n%v\n", test.desc, cmp.Diff(got, test.want))
		}
	}
}

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

func TestInvalidTransitAS(t *testing.T) {
	tests := []struct {
		desc       string
		msg        *RisMessageData
		candidates map[int32]bool
		want       bool
	}{{
		desc:       "Success - AS4 in transit position",
		msg:        msg01,
		candidates: map[int32]bool{4: true, 14: true, 0: true},
		want:       true,
	}, {
		desc:       "Success - AS10 not in transit position",
		msg:        msg01,
		candidates: map[int32]bool{10: true, 14: true, 0: true},
		want:       true,
	}}

	for _, test := range tests {
		got := test.msg.InvalidTransitAS(test.candidates)
		if got != test.want {
		}
	}
}

func TestCheckASPath(t *testing.T) {
	tests := []struct {
		desc string
		rl   *RisLive
		data *RisMessageData
		want bool
	}{{
		desc: "Success - second element",
		rl:   &RisLive{Filter: &RisFilter{ASPath: []int32{57695, 12}}},
		data: &RisMessageData{Path: []int32{57695, 12, 2332}},
		want: true,
	}, {
		desc: "Success - zero matches",
		rl:   &RisLive{Filter: &RisFilter{ASPath: []int32{57695, 12}}},
		data: &RisMessageData{Path: []int32{57695, 128, 2332}},
		want: false,
	}, {
		desc: "Success - zero to match",
		rl:   &RisLive{Filter: &RisFilter{ASPath: []int32{}}},
		data: &RisMessageData{Path: []int32{5769, 128, 2332}},
		want: true,
	}}

	for _, test := range tests {
		got := test.rl.CheckASPath(test.data)
		if got != test.want {
			t.Errorf("[%v]: got/want mismatch, wanted: %v got: %v", test.desc, test.want, got)
		}
	}
}

func TestCheckOrigins(t *testing.T) {
	tests := []struct {
		desc       string
		msg        *RisMessageData
		candidates []string
		want       bool
	}{{
		desc:       "Success found single check: 8",
		msg:        msg01,
		candidates: []string{"8"},
		want:       true,
	}, {
		desc:       "Success found double check: 8",
		msg:        msg01,
		candidates: []string{"4", "8"},
		want:       true,
	}, {
		desc:       "Failure not found single check: 4",
		msg:        msg01,
		candidates: []string{"4"},
		want:       false,
	}, {
		desc:       "Failure not found double check: 4",
		msg:        msg01,
		candidates: []string{"4", "5"},
		want:       false,
	}}

	for _, test := range tests {
		got := test.msg.CheckOrigins(test.candidates)
		if got != test.want {
			t.Errorf("[%v]: got/want mismatch got: %v want: %v", test.desc, got, test.want)
		}
	}
}

func TestCheckInvalidTransitAS(t *testing.T) {
	tests := []struct {
		desc string
		rl   *RisLive
		msg  *RisMessageData
		want bool
	}{{
		desc: "Success - Transit-AS found",
		rl:   &RisLive{Filter: &RisFilter{InvalidTransitAS: map[int32]bool{32: true, 1: true}}},
		msg:  &RisMessageData{Path: []int32{12, 701, 1, 4}},
		want: true,
	}, {
		desc: "Success - Transit-AS not found",
		rl:   &RisLive{Filter: &RisFilter{InvalidTransitAS: map[int32]bool{32: true, 1: true}}},
		msg:  &RisMessageData{Path: []int32{12, 701, 5, 4}},
		want: false,
	}, {
		desc: "Success - InvalidTransitAS is zero length - false return",
		rl:   &RisLive{Filter: &RisFilter{InvalidTransitAS: map[int32]bool{}}},
		msg:  &RisMessageData{Path: []int32{12, 701, 5, 4}},
		want: false,
	}}

	for _, test := range tests {
		got := test.rl.CheckInvalidTransitAS(test.msg)
		if got != test.want {
			t.Errorf("[%v]: got(%v)/want(%v) mismatch", test.desc, got, test.want)
		}
	}
}

// Because there are CheckOrigins in both the RisLive and RisMessageData bits.
func TestCheckOriginsRisLive(t *testing.T) {
	tests := []struct {
		desc string
		rl   *RisLive
		msg  *RisMessageData
		want bool
	}{{
		desc: "Success - Origin Match",
		rl:   &RisLive{Filter: &RisFilter{Origins: []string{"1", "701", "7018"}}},
		msg:  &RisMessageData{Origin: "701"},
		want: true,
	}, {
		desc: "Success - Origins not found - false match",
		rl:   &RisLive{Filter: &RisFilter{Origins: []string{"1", "7018", "3356"}}},
		msg:  &RisMessageData{Origin: "701"},
		want: false,
	}, {
		desc: "Success - Origins zero length - false match",
		rl:   &RisLive{Filter: &RisFilter{Origins: []string{}}},
		msg:  &RisMessageData{Origin: "701"},
		want: false,
	}}

	for _, test := range tests {
		got := test.rl.CheckOrigins(test.msg)
		if got != test.want {
			t.Errorf("[%v]: got(%v)/want(%v) mismatch", test.desc, got, test.want)
		}
	}
}

func TestCheckPrefix(t *testing.T) {
	tests := []struct {
		desc string
		rm   *RisMessageData
		rl   *RisLive
		want bool
	}{{
		desc: "Simple prefix match",
		rm: &RisMessageData{
			Announcements: []*RisAnnouncement{
				&RisAnnouncement{
					Prefixes: []string{"192.168.0.0/16"},
				},
			},
		},
		rl:   &RisLive{Filter: &RisFilter{Prefix: []string{"192.168.0.0/16"}}},
		want: true,
	}, {
		desc: "Match a subnet announcement",
		rm: &RisMessageData{
			Announcements: []*RisAnnouncement{
				&RisAnnouncement{
					Prefixes: []string{"192.168.0.0/24"},
				},
			},
		},
		rl:   &RisLive{Filter: &RisFilter{Prefix: []string{"192.168.0.0/16"}}},
		want: true,
	}, {
		desc: "RisLive data is improper",
		rm: &RisMessageData{
			Announcements: []*RisAnnouncement{
				&RisAnnouncement{
					Prefixes: []string{"192.168.0.0/24"},
				},
			},
		},
		rl:   &RisLive{Filter: &RisFilter{Prefix: []string{"192.b.0.0/16"}}},
		want: false,
	}, {
		desc: "RisMessageData is improper",
		rm: &RisMessageData{
			Announcements: []*RisAnnouncement{
				&RisAnnouncement{
					Prefixes: []string{"192.b.0.0/24"},
				},
			},
		},
		rl:   &RisLive{Filter: &RisFilter{Prefix: []string{"192.168.0.0/16"}}},
		want: false,
	}}

	for _, test := range tests {
		got := test.rl.CheckPrefix(test.rm)
		if got != test.want {
			t.Errorf("[%v]: got/want mismatch: got %v wanted %v", test.desc, got, test.want)
		}
	}
}

func TestListen(t *testing.T) {
	tests := []struct {
		desc   string
		file   *string
		recNum int
		want   RisMessage
	}{{
		/*
			desc:   "Successful read of 1 message",
			file:   proto.String("testdata/1-msg"),
			recNum: 0,
			want: RisMessage{
				Type: "ris_message",
				Data: &RisMessageData{
					Timestamp: 1.55862004708e+09,
					Peer:      "196.60.9.165",
					PeerASN:   "57695",
					ID:        "196.60.9.165-1558620047.08-11924763",
					Host:      "rrc19",
					Type:      "UPDATE",
					Path:      []int32{57695, 37650},
					Community: [][]int32{{57695, 12000}, {57695, 12001}},
					Origin:    "igp",
					Announcements: []*RisAnnouncement{
						&RisAnnouncement{
							NextHop:  "196.60.9.165",
							Prefixes: []string{"196.50.70.0/24"}}},
					Raw: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003E02000000234001010040020A02020000E15F00009312400304C43C09A5E00808E15F2EE0E15F2EE118C43246",
				}},
				}, {
		*/
		desc:   "Successful read of 6th message",
		file:   proto.String("testdata/10-msg"),
		recNum: 5,
		want: RisMessage{
			Type: "ris_message",
			Data: &RisMessageData{
				Timestamp: 1.55862004706e+09,
				Peer:      "2001:7f8:d:ff::226",
				PeerASN:   "24482",
				ID:        "2001:7f8:d:ff::226-1558620047.06-51675230",
				Host:      "rrc07",
				Type:      "UPDATE",
				Path:      []int32{24482, 6453, 174, 513, 513, 12654},
				Community: [][]int32{{6453, 86}, {6453, 1000}, {6453, 1400}, {6453, 1402}, {6453, 2000}, {6453, 4000}, {24482, 1}, {24482, 12020}, {24482, 12021}, {24482, 20200}, {24482, 20300}, {24482, 64601}},
				Origin:    "igp",
				Announcements: []*RisAnnouncement{
					&RisAnnouncement{
						NextHop:  "2001:7f8:d:ff::226",
						Prefixes: []string{"2001:7fb:fe04::/48"},
					},
					&RisAnnouncement{
						NextHop:  "fe80::2a0:a500:0:3e6",
						Prefixes: []string{"2001:7fb:fe04::/48"},
					},
				},
				Raw: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00AD02000000964001010040021A020600005FA200001935000000AE00000201000002010000316E800404000007D4C007080000FD090A1DA9C0C0083019350056193503E8193505781935057A193507D019350FA05FA200015FA22EF45FA22EF55FA24EE85FA24F4C5FA2FC59900E002C00020120200107F8000D00FF0000000000000226FE8000000000000002A0A500000003E60030200107FBFE04"},
		},
	}}

	for _, test := range tests {
		r := &RisLive{
			File:   test.file,
			Filter: &RisFilter{},
			Chan:   make(chan RisMessage, 10),
		}
		go r.Listen()

		for x := 0; x < test.recNum; x++ {
			_ = <-r.Chan
		}
		got := <-r.Chan

		if !cmp.Equal(got, test.want) {
			t.Errorf("[%v]: got/want differ(+got/-want):\n%v\n", test.desc, cmp.Diff(got, test.want))
		}
	}
}
