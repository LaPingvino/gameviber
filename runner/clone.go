package runner

import "maps"

// Clone copies the room's whole state — instances (their variables, alarms,
// timelines' positions), the step's pending room change and sleep, and the
// globals — so a copy can be stepped on its own (the play-check bot tries
// moves from saved states). Objects and timelines are definitions and are
// shared. It also returns each old instance's copy, for callers that keep
// instance pointers. The copy's globals have no sound.
func (r *Room) Clone() (*Room, map[*Instance]*Instance) {
	c := *r
	c.Instances = make([]*Instance, len(r.Instances))
	m := make(map[*Instance]*Instance, len(r.Instances))
	for i, in := range r.Instances {
		ci := *in
		ci.Vars = maps.Clone(in.Vars)
		ci.VarsStr = maps.Clone(in.VarsStr)
		c.Instances[i] = &ci
		m[in] = &ci
	}
	if r.G != nil {
		g := *r.G
		g.Vars = maps.Clone(r.G.Vars)
		g.Sound = nil
		c.G = &g
	}
	return &c, m
}
