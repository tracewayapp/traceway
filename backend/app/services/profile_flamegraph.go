package services

import (
	"sort"

	"github.com/tracewayapp/traceway/backend/app/models"
)

type FlameNode struct {
	Name     string       `json:"name"`
	Value    int64        `json:"value"`
	Self     int64        `json:"self"`
	Children []*FlameNode `json:"children,omitempty"`
}

func FoldFlameGraph(stacks []models.ProfileStackValue) *FlameNode {
	root := &FlameNode{Name: "root"}
	childIndex := map[*FlameNode]map[string]*FlameNode{}

	for _, s := range stacks {
		root.Value += s.Value
		node := root
		for i, frame := range s.Stack {
			children, ok := childIndex[node]
			if !ok {
				children = map[string]*FlameNode{}
				childIndex[node] = children
			}
			child, ok := children[frame]
			if !ok {
				child = &FlameNode{Name: frame}
				children[frame] = child
				node.Children = append(node.Children, child)
			}
			child.Value += s.Value
			if i == len(s.Stack)-1 {
				child.Self += s.Value
			}
			node = child
		}
	}

	sortFlameChildren(root)
	return root
}

func sortFlameChildren(n *FlameNode) {
	sort.SliceStable(n.Children, func(i, j int) bool {
		if n.Children[i].Value != n.Children[j].Value {
			return n.Children[i].Value > n.Children[j].Value
		}
		return n.Children[i].Name < n.Children[j].Name
	})
	for _, c := range n.Children {
		sortFlameChildren(c)
	}
}
