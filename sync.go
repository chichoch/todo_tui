package main

type conflict struct {
	id     string
	local  *Item // nil = missing/deleted on local side
	remote *Item // nil = missing/deleted on remote side
}

type resolutionKind int

const (
	resolutionLocal resolutionKind = iota
	resolutionRemote
	resolutionBoth
	resolutionAbort
)

type resolution struct {
	id   string
	kind resolutionKind
}

func itemEq(a, b Item) bool {
	return a.text == b.text && a.checked == b.checked
}

func indexByID(items []Item) map[string]Item {
	m := make(map[string]Item, len(items))
	for _, it := range items {
		m[it.id] = it
	}
	return m
}

func itemPtr(it Item) *Item {
	c := it
	return &c
}

// merge returns the auto-resolved items (in local-first order, with remote-only
// adds appended) and the list of conflicts that need user resolution.
func merge(base, local, remote []Item) ([]Item, []conflict) {
	baseByID := indexByID(base)
	remoteByID := indexByID(remote)

	var auto []Item
	var conflicts []conflict
	seen := make(map[string]bool, len(local)+len(remote))

	for _, l := range local {
		if seen[l.id] {
			continue
		}
		seen[l.id] = true
		b, hasBase := baseByID[l.id]
		r, hasRemote := remoteByID[l.id]

		switch {
		case !hasBase && !hasRemote:
			auto = append(auto, l)
		case !hasBase && hasRemote:
			if itemEq(l, r) {
				auto = append(auto, l)
			} else {
				conflicts = append(conflicts, conflict{id: l.id, local: itemPtr(l), remote: itemPtr(r)})
			}
		case hasBase && !hasRemote:
			if itemEq(l, b) {
				// remote deleted, local unchanged → accept delete
				continue
			}
			// local edited vs remote deleted
			conflicts = append(conflicts, conflict{id: l.id, local: itemPtr(l), remote: nil})
		default:
			// in all three
			localChanged := !itemEq(l, b)
			remoteChanged := !itemEq(r, b)
			switch {
			case !localChanged && !remoteChanged:
				auto = append(auto, l)
			case !localChanged && remoteChanged:
				auto = append(auto, r)
			case localChanged && !remoteChanged:
				auto = append(auto, l)
			case itemEq(l, r):
				auto = append(auto, l)
			default:
				conflicts = append(conflicts, conflict{id: l.id, local: itemPtr(l), remote: itemPtr(r)})
			}
		}
	}

	for _, r := range remote {
		if seen[r.id] {
			continue
		}
		seen[r.id] = true
		b, hasBase := baseByID[r.id]
		if !hasBase {
			auto = append(auto, r)
			continue
		}
		if itemEq(r, b) {
			// remote unchanged, local deleted → accept delete
			continue
		}
		// remote edited vs local deleted
		conflicts = append(conflicts, conflict{id: r.id, local: nil, remote: itemPtr(r)})
	}

	// IDs only in base (not in local, not in remote): both deleted → drop.

	return auto, conflicts
}

// applyResolutions appends resolved-conflict items to auto according to user choices.
// Order is conflict-list order (which itself follows local-first traversal).
func applyResolutions(auto []Item, conflicts []conflict, resolutions []resolution) []Item {
	resByID := make(map[string]resolutionKind, len(resolutions))
	for _, r := range resolutions {
		resByID[r.id] = r.kind
	}

	out := make([]Item, 0, len(auto)+len(conflicts))
	out = append(out, auto...)

	for _, c := range conflicts {
		kind, ok := resByID[c.id]
		if !ok {
			continue
		}
		switch kind {
		case resolutionLocal:
			if c.local != nil {
				out = append(out, *c.local)
			}
		case resolutionRemote:
			if c.remote != nil {
				out = append(out, *c.remote)
			}
		case resolutionBoth:
			if c.local != nil {
				out = append(out, *c.local)
			}
			if c.remote != nil {
				// Remote keeps original ID — but we're keeping both, so the
				// remote copy needs a fresh ID to avoid duplicate IDs in the file.
				rcopy := *c.remote
				rcopy.id = newItemID()
				out = append(out, rcopy)
			}
		}
	}
	return out
}
