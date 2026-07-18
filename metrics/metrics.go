package metrics

import (
	"sort"

	"depact/parser"
	"depact/resolver"
	"depact/walker"
)

type Barrel struct {
	Node          *walker.Node
	Importers     int
	Symbols       int
	Namespace     bool
	Deps          int
	Reexports     int
	UsedTargets   int
	Wasted        int
	WastedTargets []string
	Unprovable    bool
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

type Contributor struct {
	Path      string
	Exclusive int
	Subtree   int
}

func Contributors(entry *walker.Node) []Contributor {
	total := len(reachable(entry, nil))
	seen := map[*walker.Node]bool{entry: true}
	out := make([]Contributor, 0, len(entry.Edges))
	for _, e := range entry.Edges {
		if seen[e.To] {
			continue
		}
		seen[e.To] = true
		out = append(out, Contributor{
			Path:      e.To.Module.Path,
			Exclusive: total - len(reachableSkipEdge(entry, e.To)),
			Subtree:   len(reachable(e.To, nil)),
		})
	}
	return out
}

func reachableSkipEdge(entry, skip *walker.Node) map[*walker.Node]bool {
	seen := map[*walker.Node]bool{entry: true}
	queue := []*walker.Node{entry}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, e := range n.Edges {
			if n == entry && e.To == skip {
				continue
			}
			if seen[e.To] {
				continue
			}
			seen[e.To] = true
			queue = append(queue, e.To)
		}
	}
	return seen
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
	used := map[string]map[string]bool{}
	unprovable := map[string]bool{}
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
				used[p] = map[string]bool{}
			}
			b.Importers++
			if len(e.Import.Symbols) == 0 {
				unprovable[p] = true
			}
			for _, s := range e.Import.Symbols {
				if s.Kind == parser.NamespaceSymbol {
					b.Namespace = true
					unprovable[p] = true
					continue
				}
				used[p][symbolKey(s)] = true
			}
		}
	}
	for p, b := range out {
		b.Symbols = len(used[p])
		computeWaste(b, used[p], unprovable[p])
	}
	return out
}

func computeWaste(b *Barrel, used map[string]bool, unprovable bool) {
	targets, reexportFrom := reexportTargets(b.Node)
	b.Reexports = len(targets)

	keepRoots := make([]*walker.Node, 0)
	for _, e := range b.Node.Edges {
		if !reexportFrom[e.Import.From] {
			keepRoots = append(keepRoots, e.To)
		}
	}

	wasted := make([]*walker.Node, 0)
	for _, t := range targets {
		if t.local || targetUsed(t.names, used) {
			b.UsedTargets++
			keepRoots = append(keepRoots, t.node)
		} else {
			wasted = append(wasted, t.node)
		}
	}

	if unprovable {
		b.Unprovable = true
		return
	}
	if len(wasted) == 0 {
		return
	}

	keep := reachableFrom(keepRoots)
	keep[b.Node] = true
	sort.Slice(wasted, func(i, j int) bool {
		si, sj := len(reachable(wasted[i], nil)), len(reachable(wasted[j], nil))
		if si != sj {
			return si > sj
		}
		return wasted[i].Module.Path < wasted[j].Module.Path
	})
	for _, w := range wasted {
		b.WastedTargets = append(b.WastedTargets, w.Module.Path)
	}
	for n := range reachableFrom(wasted) {
		if !keep[n] {
			b.Wasted++
		}
	}
}

type reexportTarget struct {
	node  *walker.Node
	names map[string]bool
	local bool
}

func reexportTargets(barrel *walker.Node) ([]reexportTarget, map[string]bool) {
	edgeByFrom := map[string]*walker.Node{}
	for _, e := range barrel.Edges {
		edgeByFrom[e.Import.From] = e.To
	}
	importCount := map[string]int{}
	for _, imp := range barrel.Module.Imports {
		if imp.From != "" {
			importCount[imp.From]++
		}
	}
	exportCount := map[string]int{}
	for _, exp := range barrel.Module.Exports {
		if exp.From != "" {
			exportCount[exp.From]++
		}
	}

	reexportFrom := map[string]bool{}
	byNode := map[*walker.Node]*reexportTarget{}
	order := []*walker.Node{}
	for _, exp := range barrel.Module.Exports {
		if exp.From == "" {
			continue
		}
		reexportFrom[exp.From] = true
		to := edgeByFrom[exp.From]
		if to == nil {
			continue
		}
		t := byNode[to]
		if t == nil {
			t = &reexportTarget{node: to, names: map[string]bool{}}
			byNode[to] = t
			order = append(order, to)
		}
		if importCount[exp.From] > exportCount[exp.From] {
			t.local = true
		}
		for _, s := range exp.Symbols {
			if s.Kind == parser.NamespaceSymbol {
				t.names["*"] = true
				continue
			}
			t.names[exportKey(s)] = true
		}
	}
	targets := make([]reexportTarget, len(order))
	for i, n := range order {
		targets[i] = *byNode[n]
	}
	return targets, reexportFrom
}

func targetUsed(names, used map[string]bool) bool {
	if names["*"] {
		return true
	}
	for name := range names {
		if used[name] {
			return true
		}
	}
	return false
}

func reachableFrom(roots []*walker.Node) map[*walker.Node]bool {
	seen := map[*walker.Node]bool{}
	queue := make([]*walker.Node, 0, len(roots))
	for _, r := range roots {
		if !seen[r] {
			seen[r] = true
			queue = append(queue, r)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, e := range n.Edges {
			if !seen[e.To] {
				seen[e.To] = true
				queue = append(queue, e.To)
			}
		}
	}
	return seen
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

func exportKey(s parser.Symbol) string {
	if s.Kind == parser.DefaultSymbol {
		return "default"
	}
	if s.Alias != "" {
		return s.Alias
	}
	return s.Name
}
