package services

import (
	"sort"

	"github.com/tracewayapp/traceway/backend/app/models"
)

const (
	maxFlameDepth = 256
	maxFlameNodes = 60000
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
	nodeCount := 0

	for _, s := range stacks {
		root.Value += s.Value
		node := root
		depth := len(s.Stack)
		if depth > maxFlameDepth {
			depth = maxFlameDepth
		}
		for i := 0; i < depth; i++ {
			frame := s.Stack[i]
			children, ok := childIndex[node]
			if !ok {
				children = map[string]*FlameNode{}
				childIndex[node] = children
			}
			child, ok := children[frame]
			if !ok {
				if nodeCount >= maxFlameNodes {
					break
				}
				child = &FlameNode{Name: frame}
				children[frame] = child
				node.Children = append(node.Children, child)
				nodeCount++
			}
			child.Value += s.Value
			if i == depth-1 {
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
