package control

import (
	"reflect"
	"testing"
)

func TestGroupPresentationPersistsWithoutChangingRouting(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	first := createGroupWithCredentials(t, fixture, "sk-presentation-a")
	second := createGroupWithCredentials(t, fixture, "sk-presentation-b")
	snapshot := fixture.manager.Current()
	order := []uint{second, first}
	saved, err := fixture.service.UpdateGroupPresentation(t.Context(), GroupPresentationUpdate{Order: &order})
	if err != nil || !reflect.DeepEqual(saved.Order, order) {
		t.Fatalf("save order = %#v, %v", saved, err)
	}
	icon := "builtin:mistral"
	if _, err := fixture.service.UpdateGroupPresentation(t.Context(), GroupPresentationUpdate{GroupID: &first, Icon: &icon}); err != nil {
		t.Fatal(err)
	}
	loaded, err := fixture.service.GetGroupPresentation(t.Context())
	if err != nil || !reflect.DeepEqual(loaded.Order, order) || loaded.Icons[first] != icon {
		t.Fatalf("load presentation = %#v, %v", loaded, err)
	}
	if fixture.manager.Current() != snapshot {
		t.Fatal("presentation changed routing snapshot")
	}
	invalid := []uint{first, first}
	if _, err := fixture.service.UpdateGroupPresentation(t.Context(), GroupPresentationUpdate{Order: &invalid}); err == nil {
		t.Fatal("duplicate IDs accepted")
	}
	unsafe := "javascript:alert(1)"
	if _, err := fixture.service.UpdateGroupPresentation(t.Context(), GroupPresentationUpdate{GroupID: &first, Icon: &unsafe}); err == nil {
		t.Fatal("unsafe icon accepted")
	}
	unchanged, err := fixture.service.GetGroupPresentation(t.Context())
	if err != nil || !reflect.DeepEqual(unchanged, loaded) {
		t.Fatal("rejected edit changed presentation")
	}
}
