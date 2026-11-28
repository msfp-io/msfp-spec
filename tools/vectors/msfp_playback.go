package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"msfp-media/tools/vectors/rules"
)

type playbackKeptLimits struct {
	Streams         *int64 `json:"streams,omitempty"`
	Transcodes      *int64 `json:"transcodes,omitempty"`
	BitrateKbps     *int64 `json:"bitrateKbps,omitempty"`
	Downloads       int64  `json:"downloads"`
	DownloadsPerDay int64  `json:"downloadsPerDay"`
	DownloadKbps    int64  `json:"downloadKbps"`
	ScansPerDay     int64  `json:"scansPerDay"`
	AudioStreams    *int64 `json:"audioStreams,omitempty"`
	AudioTranscodes *int64 `json:"audioTranscodes,omitempty"`
	OffersDownloads bool   `json:"offersDownloads"`
	OffersScans     bool   `json:"offersScans"`
}

// playbackKeepLimits reads the libraries answer's limits: a limits that
// isn't an object, or one of whose known members isn't an integer, is
// ignored whole (an error); a member out of range is ignored alone.
// scanBytes is no member of limits now (a scan's answer states it):
// unknown, it is ignored whatever it holds.
func playbackKeepLimits(raw []byte) ([]byte, error) {
	o, err := playbackDecode(raw)
	if err != nil {
		return nil, err
	}
	known := []struct {
		name string
		max  int64
		min  int64
	}{{"streams", playbackMaxStreams, 0}, {"transcodes", playbackMaxStreams, 0}, {"bitrateKbps", 1000000, 0}, {"downloads", playbackMaxStreams, 0}, {"downloadsPerDay", 10000, 0}, {"downloadKbps", 1000000, 0}, {"scansPerDay", 1000000, 0}, {"audioStreams", playbackMaxStreams, 0}, {"audioTranscodes", playbackMaxStreams, 0}}
	got := map[string]int64{}
	for _, k := range known {
		v, present := o[k.name]
		if !present {
			continue
		}
		n, ok := playbackInt(v)
		if !ok {
			return nil, fmt.Errorf("%s isn't an integer: limits ignored", k.name)
		}
		if n >= k.min && n <= k.max {
			got[k.name] = n
		}
	}
	// transcodes 0 to streams, audioTranscodes 0 to audioStreams.
	for _, pair := range [][2]string{{"transcodes", "streams"}, {"audioTranscodes", "audioStreams"}} {
		if n, ok := got[pair[0]]; ok {
			if top, ok := got[pair[1]]; ok && n > top {
				delete(got, pair[0])
			}
		}
	}
	ptr := func(name string) *int64 {
		if n, ok := got[name]; ok {
			return &n
		}
		return nil
	}
	k := playbackKeptLimits{
		Streams: ptr("streams"), Transcodes: ptr("transcodes"), BitrateKbps: ptr("bitrateKbps"),
		Downloads: got["downloads"], DownloadsPerDay: got["downloadsPerDay"], DownloadKbps: got["downloadKbps"], ScansPerDay: got["scansPerDay"],
		AudioStreams: ptr("audioStreams"), AudioTranscodes: ptr("audioTranscodes"),
	}
	k.OffersDownloads, k.OffersScans = k.Downloads > 0, k.ScansPerDay > 0
	return playbackEncode(k), nil
}

// MSFP's playback part (spec/v0/playback.md) on top of MSFP Media's
// playing (the rules package): the purpose scan, the refusal of a
// capability list, the refusal over a pairing's limits, and the context of
// playback.json, with scanBytes.

// msfpLimitRefusal is MSFP's refusal over a pairing's limits (playback
// section 2).
var msfpLimitRefusal = rules.LimitRefusal{Status: 409, Type: "stream-limit-reached"}

const (
	playbackScanTTL   = 600
	playbackScanFloor = 4 << 20 // scanBytes absent or out of range
	playbackScanTop   = 1 << 30
)

// msfpCarrier is MSFP as playing's carrier: no capability list (playback
// section 2), and the purpose scan (section 8), whose answer states
// scanBytes, the pool's scan cap (0: none stated).
func msfpCarrier(scanBytes int64) *rules.Carrier {
	return &rules.Carrier{
		Capabilities: "capabilities: MSFP takes no capability list",
		Purposes: map[string]rules.CarrierPurpose{"scan": {
			Allowed: func(delivery string, _ *rules.Targets) bool { return delivery == "file" },
			Zero: func(lim rules.Limits) string {
				if rules.Count(lim.Own, "scansPerDay") == 0 {
					return "scansPerDay is 0"
				}
				return ""
			},
			Start: func(s *rules.Started, lim rules.Limits, use rules.Use, progress int64) string {
				if rules.Count(use.Own, "scansToday") >= rules.Count(lim.Own, "scansPerDay") {
					return "scansPerDay reached"
				}
				s.Counts, s.TicketSeconds, s.ProgressSeconds, s.IdleSeconds = "scansPerDay", playbackScanTTL, progress, 2*progress
				if scanBytes != 0 {
					s.Carried = []rules.Member{{Key: "scanBytes", Value: scanBytes}}
				}
				return ""
			},
			Answer: func(o map[string]any) []rules.Member {
				n, ok := playbackInt(o["scanBytes"])
				if !ok || n < playbackScanFloor || n > playbackScanTop {
					n = playbackScanFloor
				}
				return []rules.Member{{Key: "scanBytes", Value: n}}
			},
			Silent: true,
		}},
	}
}

// msfpContext is MSFP Media's context as playback.json gives it: the
// pool's scan cap stated as scanBytes, and no limitRefusal (MSFP's is
// msfpLimitRefusal).
type msfpContext struct {
	rules.Context
	ScanBytes int64
}

func (c msfpContext) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(c.Context)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	d := json.NewDecoder(bytes.NewReader(b))
	if err := d.Decode(&m); err != nil {
		return nil, err
	}
	// Rebuilt in the context's order, scanBytes before progressSeconds.
	var keys []string
	d = json.NewDecoder(bytes.NewReader(b))
	d.Token()
	for d.More() {
		k, _ := d.Token()
		var raw json.RawMessage
		d.Decode(&raw)
		keys = append(keys, k.(string))
	}
	out := []byte{'{'}
	for _, k := range keys {
		if k == "limitRefusal" {
			continue
		}
		if k == "progressSeconds" {
			out = append(out, rules.Members([]rules.Member{{Key: "scanBytes", Value: c.ScanBytes}})...)
			out = append(out, ',')
		}
		out = append(append(append(out, playbackEncode(k)...), ':'), m[k]...)
		out = append(out, ',')
	}
	out[len(out)-1] = '}'
	return out, nil
}

func (c *msfpContext) UnmarshalJSON(b []byte) error {
	var s struct {
		ScanBytes int64 `json:"scanBytes"`
	}
	if err := json.Unmarshal(b, &c.Context); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	c.ScanBytes = s.ScanBytes
	c.LimitRefusal = msfpLimitRefusal
	return nil
}

// playbackContextOf is MSFP Media's context of the cases, as MSFP carries
// them: the pool's scan cap 32 MiB.
func playbackContextOf() msfpContext {
	c := rules.ContextOf()
	c.LimitRefusal = msfpLimitRefusal
	// The counters of the purpose scan (playback section 5).
	c.Limits.Own = []rules.Counter{{Name: "scansPerDay", N: 1000}}
	c.InUse.Own = []rules.Counter{{Name: "scansToday", N: 0}}
	return msfpContext{Context: c, ScanBytes: 33554432}
}

// pbLim and pbUse are the context's limits and what is in use, as f
// changes them.
func pbLim(f func(*playbackLimits)) *playbackLimits {
	l := playbackContextOf().Limits
	f(&l)
	return &l
}

func pbUse(f func(*playbackUse)) *playbackUse {
	u := playbackContextOf().InUse
	f(&u)
	return &u
}

// pbIn is a case before its outcome is worked out, as MSFP Media's
// (rules.In) with a scan's cap.
type pbIn struct {
	name, text string
	limits     *playbackLimits
	inUse      *playbackUse
	ranges     []string
	scanBytes  int64
	progress   int64
	playback   *playbackState
	call       string
}

// pbJudge works out each case's outcome with MSFP Media's rules, MSFP
// carrying them.
func pbJudge(ctx msfpContext, in []pbIn) []playbackCase {
	var out []playbackCase
	for _, c := range in {
		pc := playbackCase{Name: c.name, Call: c.call, Limits: c.limits, InUse: c.inUse, Ranges: c.ranges, ScanBytes: c.scanBytes, Progress: c.progress, Playback: c.playback, Text: c.text}
		if pc.Call == "" {
			pc.Call = "play"
		}
		got, err := pbRun(ctx, pc)
		pc.Valid, pc.Reason = reason(err)
		pc.Output = string(got)
		pc.Status, pc.Type = playbackStatus(err)
		out = append(out, pc)
	}
	return out
}

// pbRun applies the rule a case names, MSFP carrying it with the case's
// scan cap, else the context's; verify_rev_playback.go calls it too.
func pbRun(ctx msfpContext, c playbackCase) ([]byte, error) {
	scan := ctx.ScanBytes
	if c.ScanBytes != 0 {
		scan = c.ScanBytes
	}
	rc := ctx.Context
	rc.Carrier = msfpCarrier(scan)
	return rules.Run(rc, rules.Case{Name: c.Name, Call: c.Call, Limits: c.Limits, InUse: c.InUse, Ranges: c.Ranges, Progress: c.Progress, Playback: c.Playback, Text: c.Text})
}

// mediaServes is MSFP Media's rule of the names a ticket opens (fetching
// section 1), with MSFP's scan ticket, which opens file only (media
// section 4.2).
func mediaServes(t mediaTicket, name string) error {
	if t.Kind == "scan" {
		if name == "file" {
			return nil
		}
		return errors.New("a scan ticket opens file only")
	}
	return rules.MediaServes(t, name)
}
