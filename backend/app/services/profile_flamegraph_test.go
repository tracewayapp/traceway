package services

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func flameChild(n *FlameNode, name string) *FlameNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestFoldFlameGraph_SingleStack(t *testing.T) {
	root := FoldFlameGraph([]models.ProfileStackValue{
		{Stack: []string{"a", "b", "c"}, Value: 10},
	})

	if root.Name != "root" || root.Value != 10 {
		t.Fatalf("root = %+v, want name=root value=10", root)
	}
	a := flameChild(root, "a")
	if a == nil || a.Value != 10 || a.Self != 0 {
		t.Fatalf("a = %+v, want value=10 self=0", a)
	}
	b := flameChild(a, "b")
	if b == nil || b.Value != 10 || b.Self != 0 {
		t.Fatalf("b = %+v, want value=10 self=0", b)
	}
	c := flameChild(b, "c")
	if c == nil || c.Value != 10 || c.Self != 10 {
		t.Fatalf("c = %+v, want value=10 self=10 (leaf carries self)", c)
	}
}

func TestFoldFlameGraph_SharedPrefix(t *testing.T) {
	root := FoldFlameGraph([]models.ProfileStackValue{
		{Stack: []string{"a", "b"}, Value: 10},
		{Stack: []string{"a", "c"}, Value: 5},
	})

	if root.Value != 15 {
		t.Fatalf("root.Value = %d, want 15", root.Value)
	}
	a := flameChild(root, "a")
	if a == nil || a.Value != 15 || a.Self != 0 {
		t.Fatalf("a = %+v, want cumulative 15 self 0", a)
	}
	if b := flameChild(a, "b"); b == nil || b.Value != 10 || b.Self != 10 {
		t.Fatalf("b = %+v, want value 10 self 10", b)
	}
	if c := flameChild(a, "c"); c == nil || c.Value != 5 || c.Self != 5 {
		t.Fatalf("c = %+v, want value 5 self 5", c)
	}
}

func TestFoldFlameGraph_PrefixIsAlsoLeaf(t *testing.T) {
	root := FoldFlameGraph([]models.ProfileStackValue{
		{Stack: []string{"a"}, Value: 3},
		{Stack: []string{"a", "b"}, Value: 7},
	})

	a := flameChild(root, "a")
	if a == nil || a.Value != 10 || a.Self != 3 {
		t.Fatalf("a = %+v, want cumulative 10 self 3", a)
	}
	if b := flameChild(a, "b"); b == nil || b.Value != 7 || b.Self != 7 {
		t.Fatalf("b = %+v, want value 7 self 7", b)
	}
}

func TestFoldFlameGraph_ChildrenOrderedByValueDesc(t *testing.T) {
	root := FoldFlameGraph([]models.ProfileStackValue{
		{Stack: []string{"a"}, Value: 1},
		{Stack: []string{"b"}, Value: 5},
		{Stack: []string{"c"}, Value: 3},
	})

	var order []string
	for _, c := range root.Children {
		order = append(order, c.Name)
	}
	if len(order) != 3 || order[0] != "b" || order[1] != "c" || order[2] != "a" {
		t.Fatalf("child order = %v, want [b c a] (value desc)", order)
	}
}
