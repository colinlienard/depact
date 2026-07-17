package metrics

import (
	"depact/parser"
	"depact/resolver"
	"depact/walker"
)

type Barrel struct {
	Node      *walker.Node
	Importers int
	Symbols   int
	Namespace bool
	Deps      int
}

func Closure(g *walker.Graph) map[string]int {
	out := make(map[string]int, len(g.Modules))
	for p, n := range g.Modules {
		out[p] = len(reachable(n, nil)) - 1
	}
	return out
}

func Exclusive(g *walker.Graph, entry *walker.Node) map[string]int {
	total := reachable(entry, nil)
	out := make(map[string]int, len(total)-1)
	for n := range total {
		if n == entry {
			continue
		}
		out[n.Module.Path] = len(total) - len(reachable(entry, n))
	}
	return out
}

func Why(g *walker.Graph, entry *walker.Node, target string) []*walker.Node {
	dest := g.Modules[target]
	if dest == nil || entry == nil {
		return nil
	}
	prev := map[*walker.Node]*walker.Node{entry: nil}
	queue := []*walker.Node{entry}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if n == dest {
			var chain []*walker.Node
			for ; n != nil; n = prev[n] {
				chain = append([]*walker.Node{n}, chain...)
			}
			return chain
		}
		for _, e := range n.Edges {
			if _, seen := prev[e.To]; seen {
				continue
			}
			prev[e.To] = n
			queue = append(queue, e.To)
		}
	}
	return nil
}

func Reach(n *walker.Node) map[*walker.Node]bool {
	return reachable(n, nil)
}

func Barrels(g *walker.Graph) map[string]*Barrel {
	out := map[string]*Barrel{}
	symbols := map[string]map[string]bool{}
	for _, n := range g.Modules {
		for _, e := range n.Edges {
			if e.Kind != resolver.ResolveKindIndex {
				continue
			}
			p := e.To.Module.Path
			b := out[p]
			if b == nil {
				b = &Barrel{Node: e.To, Deps: len(reachable(e.To, nil)) - 1}
				out[p] = b
				symbols[p] = map[string]bool{}
			}
			b.Importers++
			for _, s := range e.Import.Symbols {
				if s.Kind == parser.NamespaceSymbol {
					b.Namespace = true
					continue
				}
				symbols[p][symbolKey(s)] = true
			}
		}
	}
	for p, b := range out {
		b.Symbols = len(symbols[p])
	}
	return out
}

func reachable(start, skip *walker.Node) map[*walker.Node]bool {
	seen := map[*walker.Node]bool{start: true}
	queue := []*walker.Node{start}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, e := range n.Edges {
			if e.To == skip || seen[e.To] {
				continue
			}
			seen[e.To] = true
			queue = append(queue, e.To)
		}
	}
	return seen
}

func symbolKey(s parser.Symbol) string {
	if s.Kind == parser.DefaultSymbol {
		return "default"
	}
	return s.Name
}
