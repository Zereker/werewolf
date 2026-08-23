package hiddenrole

import (
	"reflect"
	"strings"
	"testing"
)

// deliberateProjections lists the free-form state allowed to appear in a
// player-facing struct.
//
// There is exactly one: PlayerView.RoleInfo. It is what a role projects
// **explicitly** (see RoleInfoProvider), the role decides what goes in, and
// so it is a deliberate disclosure.
var deliberateProjections = map[string]bool{
	"PlayerView.RoleInfo": true,
}

// TestPlayerView_CarriesNoFreeFormState: a player-facing struct may hold no
// free-form state bag.
//
// "What goes into it is up to the role, and handing it to the player by
// default would make every role work out for itself whether each entry may be
// shown" -- that is written on PlayerInfo.Vars, and it is the entire reason
// the three faces are kept apart (the god's-view PlayerInfo, one's own
// SelfInfo, everyone else's PublicPlayerInfo).
//
// And it used to be **only a comment**: anyone adding a
// `Vars map[string]string` to SelfInfo or PublicPlayerInfo would send how
// many potions the witch has left and who the guard protected last round to
// the whole table, and nothing would make a sound. This test walks
// PlayerView's entire type graph and treats any map[string]string, RoleInfo
// excepted, as a leak.
//
// It checks shape, not values: with the shape right, a leak has to be
// something somebody filled in **explicitly**, and that step is already
// watched by the tests in player_view_test.go.
func TestPlayerView_CarriesNoFreeFormState(t *testing.T) {
	var leaks []string
	walkFields(reflect.TypeOf(PlayerView{}), "PlayerView", map[reflect.Type]bool{}, func(path string, f reflect.StructField) {
		if deliberateProjections[path] {
			return
		}
		if isFreeFormState(f.Type) {
			leaks = append(leaks, path+" "+f.Type.String())
		}
	})

	if len(leaks) > 0 {
		t.Errorf("a free-form state bag appeared in a player-facing struct:\n  %s\n"+
			"the contents of such a field are the rules' own, and the kernel "+
			"cannot judge which entries a player may see. Project what a player "+
			"should get explicitly through a RoleInfoProvider; if this really is "+
			"a deliberate disclosure, add a line to deliberateProjections with "+
			"the reason.",
			strings.Join(leaks, "\n  "))
	}
}

// TestPlayerView_ShapeTestActuallyWalks: the shape test itself has to have
// actually reached something.
//
// Walking a type graph by reflection is easy to short-circuit with one early
// return, after which it checks nothing and is green forever.
func TestPlayerView_ShapeTestActuallyWalks(t *testing.T) {
	seen := map[string]bool{}
	walkFields(reflect.TypeOf(PlayerView{}), "PlayerView", map[reflect.Type]bool{}, func(path string, _ reflect.StructField) {
		seen[path] = true
	})

	for _, want := range []string{
		"PlayerView.RoleInfo",      // the one that is allowed
		"PlayerView.Self.Camp",     // one level of nesting
		"PlayerView.Players.Role",  // through a slice
		"PlayerView.AllowedSkills", // the slice itself
	} {
		if !seen[want] {
			t.Errorf("the type walk never reached %s -- this shape test may be checking nothing", want)
		}
	}
}

// walkFields walks a struct's fields recursively, calling visit on each.
// path looks like PlayerView.Self.Camp; slices and pointers add no marker of
// their own.
func walkFields(t reflect.Type, path string, seen map[reflect.Type]bool, visit func(string, reflect.StructField)) {
	t = deref(t)
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	defer delete(seen, t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // an unexported field cannot leave the package, so it is not on the boundary
		}
		sub := path + "." + f.Name
		visit(sub, f)
		walkFields(f.Type, sub, seen, visit)
	}
}

// deref strips pointers and slices down to the element type underneath.
func deref(t reflect.Type) reflect.Type {
	for {
		switch t.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Array:
			t = t.Elem()
		default:
			return t
		}
	}
}

// isFreeFormState reports whether a type is a bag the rules can put anything
// into.
func isFreeFormState(t reflect.Type) bool {
	t = deref(t)
	return t.Kind() == reflect.Map
}
