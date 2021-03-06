package beatnik

// Parser of text format.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	hitToken = regexp.MustCompile("^\\(?([0-9A-Z]+(?:\\+*|-*)" +
		"(?:,[0-9A-Z]+(?:\\+*|-*))*)((?:\\.*|~*)>?)\\)?$")
	noteToken      = regexp.MustCompile("^([0-9A-Z]+)(\\+*|-*)$")
	waitToken      = regexp.MustCompile("^(?:\\.*|~*)>?$")
	directiveToken = regexp.MustCompile("^([^:]+):(.*)$")
	tokenizer      = regexp.MustCompile("(?m)\\s+")
	comment        = regexp.MustCompile("#[^\n]*")

	// Maps textual representation of notes to byte values.
	drumNotes = map[string]byte{}

	// Maps +- notation to actual velocities.
	velocities = map[string]Velocity{
		"-----": PPP,
		"----":  PP,
		"---":   P,
		"--":    MP,
		"-":     MF,
		"":      F,
		"+":     FF,
		"++":    FFF,
	}

	// Maps notation to note duration in ticks.
	durations = map[string]uint{
		"~~":    96 * 4,
		"~":     96 * 2,
		"":      96,
		".":     96 / 2,
		"..":    96 / 4,
		"...":   96 / 8,
		"....":  96 / 16,
		".....": 96 / 32,
	}

	// Maps directive name (in text syntax) to its handler.
	directives = map[string]directive{
		"bpm": bpmDirective,
	}
)

func init() {
	// Initialize drumNotes with mapping from string to byte ("38": byte(38)).
	byteMax := int(^byte(0))
	for i := 1; i <= byteMax; i++ {
		drumNotes[fmt.Sprint(i)] = byte(i)
	}
	for k, v := range ezDrummer {
		drumNotes[k] = v
	}

	// Add triplets to durations.
	for d := range durations {
		durations[d+">"] = durations[d] * 2 / 3
	}
}

// ParseTrack parses hit notations separated by whitespaces.
func ParseTrack(s string) (*Track, error) {
	t := &Track{}
	for i, token := range tokenize(s) {
		switch {
		case hitToken.MatchString(token):
			if halfParenthesized(token) {
				return nil, fmt.Errorf(
					"token #%v: grace notes should have parenthesis on both sides", i)
			}

			// Check for grace.
			grace := false
			if parenthesized(token) {
				grace = true
				token = token[1 : len(token)-1]
			}

			// Parse hit.
			h, err := parseHit(token)
			if err != nil {
				return nil, fmt.Errorf("token #%v: %v", i, err)
			}

			if grace {
				// Shorten last hit.
				if len(t.Hits) > 0 {
					last := t.Hits[len(t.Hits)-1]
					if last.T <= h.T {
						return nil, fmt.Errorf("token #%v: grace note is too long: "+
							"%v ticks, should be less than %v",
							i, h.T, last.T)
					}
					last.T -= h.T
				}
			}

			t.Hits = append(t.Hits, h)
		case waitToken.MatchString(token):
			d := durations[token]
			if d == 0 {
				return nil, fmt.Errorf("token #%v: bad duration: %q", i+1, token)
			}
			if len(t.Hits) == 0 {
				return nil, fmt.Errorf("token #%v: duration with no preceding note", i+1)
			}
			t.Hits[len(t.Hits)-1].T += d
		case directiveToken.MatchString(token):
			if err := t.parseDirective(token); err != nil {
				return nil, fmt.Errorf("token #%v: %v", i, err)
			}
		default:
			return nil, fmt.Errorf("token #%v: unrecognized token: %q", i+1, token)
		}
	}
	return t, nil
}

// tokenize extracts tokens from a text and returns them in a slice.
// Comments are removed.
func tokenize(s string) []string {
	s = comment.ReplaceAllString(s, "")
	var result []string
	for _, t := range tokenizer.Split(s, -1) {
		if t == "" {
			continue
		}
		result = append(result, t)
	}
	return result
}

// parseHit parses a single hit token and returns the constructed hit.
func parseHit(s string) (*Hit, error) {
	m := hitToken.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("bad hit: %q", s)
	}

	notes, err := parseNotes(m[1])
	if err != nil {
		return nil, err
	}

	d := durations[m[2]]
	if d == 0 {
		return nil, fmt.Errorf("bad duration: %q", m[2])
	}

	return &Hit{notes, d}, nil
}

// parseNotes parses the notes section of a hit token.
func parseNotes(s string) (map[byte]Velocity, error) {
	notes := map[byte]Velocity{}

	for _, part := range strings.Split(s, ",") {
		m := noteToken.FindStringSubmatch(part)
		if m == nil {
			return nil, fmt.Errorf("bad note token: %q", part)
		}

		note, v := drumNotes[m[1]], velocities[m[2]]
		if note == 0 {
			return nil, fmt.Errorf("bad drum number: %q", m[1])
		}
		if v == 0 {
			return nil, fmt.Errorf("bad velocity: %q", m[2])
		}
		notes[note] = v
	}

	return notes, nil
}

// parenthesized returns true if s starts and ends with parenthesis.
func parenthesized(s string) bool {
	return len(s) > 0 && s[0] == '(' && s[len(s)-1] == ')'
}

// halfParenthesized returns true if s only starts or only ends with parenthesis.
func halfParenthesized(s string) bool {
	return len(s) > 0 &&
		((s[0] == '(' && s[len(s)-1] != ')') ||
			(s[0] != '(' && s[len(s)-1] == ')'))
}

// A directive is a function that alters the track itself.
type directive func(*Track, string) error

// parseDirective parses a directive token and runs it.
func (t *Track) parseDirective(s string) error {
	m := directiveToken.FindStringSubmatch(s)
	if m == nil {
		return fmt.Errorf("bad directive: %q", s)
	}
	d := directives[m[1]]
	if d == nil {
		return fmt.Errorf("unknown directive: %q", m[1])
	}
	return d(t, m[2])
}

// bpmDirective changes a track's bpm.
func bpmDirective(t *Track, s string) error {
	bpm, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("bad input to BPM: %v", err)
	}
	if bpm < 1 || bpm > 500 {
		return fmt.Errorf("bad BPM: %v, must be between 1 and 500", bpm)
	}
	t.BPM = uint(bpm)
	return nil
}
