package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// PageNode represents a single node in the page tree output.
type PageNode struct {
	Handle   string `json:"handle"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	Depth    int    `json:"depth"`
	Value    string `json:"value,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
	Focused  bool   `json:"focused,omitempty"`
	NodeID   int64  `json:"-"`
}

// HandleMap stores the mapping from @N handles to backend DOM node IDs.
type HandleMap struct {
	URL     string           `json:"url"`
	Title   string           `json:"title"`
	Handles map[string]int64 `json:"handles"`
	Saved   time.Time        `json:"saved"`
}

// actionableRoles is the set of roles considered actionable for --actions-only.
var actionableRoles = map[string]bool{
	"button":           true,
	"link":             true,
	"textbox":          true,
	"checkbox":         true,
	"radio":            true,
	"combobox":         true,
	"listbox":          true,
	"menuitem":         true,
	"menuitemcheckbox": true,
	"menuitemradio":    true,
	"option":           true,
	"searchbox":        true,
	"slider":           true,
	"spinbutton":       true,
	"switch":           true,
	"tab":              true,
	"treeitem":         true,
}

// skipRoles are roles to always skip in the tree output.
var skipRoles = map[string]bool{
	"none":          true,
	"generic":       true,
	"InlineTextBox": true,
}

// --- Lenient types for raw CDP Accessibility.getFullAXTree response ---

// axNode is a lenient representation of a CDP AXNode that won't fail on
// unknown enum values (e.g. PropertyName "uninteresting").
type axNode struct {
	NodeID           string          `json:"nodeId"`
	Ignored          bool            `json:"ignored"`
	Role             *axVal          `json:"role"`
	Name             *axVal          `json:"name"`
	RawValue         *axVal          `json:"value"`
	Properties       []axProp        `json:"properties"`
	ChildIDs         []string        `json:"childIds"`
	BackendDOMNodeID int64           `json:"backendDOMNodeId"`
	IgnoredReasons   json.RawMessage `json:"ignoredReasons,omitempty"` // accept anything
}

// axVal holds a CDP AXValue with raw JSON so we can parse flexibly.
type axVal struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func (v *axVal) str() string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(v.Value, &s); err == nil {
		return s
	}
	return strings.Trim(string(v.Value), `"`)
}

func (v *axVal) boolean() bool {
	if v == nil || len(v.Value) == 0 {
		return false
	}
	var b bool
	if err := json.Unmarshal(v.Value, &b); err == nil {
		return b
	}
	return false
}

// axProp holds a single AX property with Name as a plain string.
type axProp struct {
	Name  string `json:"name"` // plain string — accepts any property name
	Value *axVal `json:"value"`
}

// doPage fetches the accessibility tree via raw CDP and outputs it with @N handles.
func doPage(sess *Session, actionsOnly bool) {
	ctx, cancel := context.WithTimeout(sess.Ctx, 15*time.Second)
	defer cancel()

	var title, location string
	chromedp.Run(ctx,
		chromedp.Title(&title),
		chromedp.Location(&location),
	)

	// Raw CDP call — bypasses cdproto's strict enum unmarshaling.
	var raw json.RawMessage
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.FromContext(ctx).Target.Execute(ctx,
			"Accessibility.getFullAXTree", nil, &raw)
	}))
	if err != nil {
		emitError(fmt.Sprintf("Failed to get accessibility tree: %v", err), "PAGE_ERROR",
			"Run `agios browser go <url>` to load a page first",
		)
		os.Exit(1)
	}

	var resp struct {
		Nodes []axNode `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		emitError(fmt.Sprintf("Failed to parse accessibility tree: %v", err), "PAGE_ERROR",
			"Run `agios browser go <url>` to load a page first",
		)
		os.Exit(1)
	}

	// Build parent→children map
	type nodeInfo struct {
		node     axNode
		children []string
	}
	nodeMap := make(map[string]*nodeInfo)
	for _, n := range resp.Nodes {
		ni := &nodeInfo{node: n, children: n.ChildIDs}
		nodeMap[n.NodeID] = ni
	}

	// Walk tree from root, building flat list with depth
	var pageNodes []PageNode
	handleCounter := 0
	handles := make(map[string]int64)

	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		ni, ok := nodeMap[id]
		if !ok {
			return
		}
		n := ni.node

		role := n.Role.str()

		if n.Ignored {
			for _, cid := range ni.children {
				walk(cid, depth)
			}
			return
		}

		if skipRoles[role] {
			for _, cid := range ni.children {
				walk(cid, depth)
			}
			return
		}

		name := n.Name.str()

		if role == "StaticText" && name == "" {
			return
		}

		if actionsOnly && !actionableRoles[role] {
			for _, cid := range ni.children {
				walk(cid, depth)
			}
			return
		}

		handleCounter++
		handle := fmt.Sprintf("@%d", handleCounter)

		pn := PageNode{
			Handle: handle,
			Role:   role,
			Name:   name,
			Depth:  depth,
			NodeID: n.BackendDOMNodeID,
		}

		// Value
		if n.RawValue != nil {
			pn.Value = n.RawValue.str()
		}

		// Properties — only check what we care about
		for _, prop := range n.Properties {
			switch prop.Name {
			case "disabled":
				if prop.Value.boolean() {
					pn.Disabled = true
				}
			case "focused":
				if prop.Value.boolean() {
					pn.Focused = true
				}
			}
		}

		handles[handle] = n.BackendDOMNodeID
		pageNodes = append(pageNodes, pn)

		for _, cid := range ni.children {
			walk(cid, depth+1)
		}
	}

	if len(resp.Nodes) > 0 {
		walk(resp.Nodes[0].NodeID, 0)
	}

	// Save handle map
	hm := HandleMap{
		URL:     location,
		Title:   title,
		Handles: handles,
		Saved:   time.Now(),
	}
	if hp, err := handlesPath(); err == nil {
		data, _ := json.MarshalIndent(hm, "", "  ")
		os.WriteFile(hp, data, 0644)
	}

	// Build compact text representation
	var lines []string
	for _, pn := range pageNodes {
		indent := strings.Repeat("  ", pn.Depth)
		line := fmt.Sprintf("%s%s %s", indent, pn.Handle, pn.Role)
		if pn.Name != "" {
			line += fmt.Sprintf(" %q", pn.Name)
		}
		if pn.Value != "" {
			line += fmt.Sprintf(" value=%q", pn.Value)
		}
		if pn.Disabled {
			line += " disabled"
		}
		if pn.Focused {
			line += " focused"
		}
		lines = append(lines, line)
	}

	emitResult(map[string]any{
		"url":   location,
		"title": title,
		"tree":  strings.Join(lines, "\n"),
		"count": len(pageNodes),
		"help": []string{
			"Use @N handles with click, input, set, hover, pick commands",
			"Run `agios browser page --actions-only` to see only interactive elements",
		},
	})
}

// loadHandles reads the handle map from disk.
func loadHandles() (*HandleMap, error) {
	hp, err := handlesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(hp)
	if err != nil {
		return nil, fmt.Errorf("no handle map found. Run `agios browser page` first")
	}

	var hm HandleMap
	if err := json.Unmarshal(data, &hm); err != nil {
		return nil, fmt.Errorf("invalid handle map: %w", err)
	}

	return &hm, nil
}

// resolveHandle looks up a handle in the cached handle map and returns the backendDOMNodeId.
func resolveHandle(handle string) (int64, error) {
	if !strings.HasPrefix(handle, "@") {
		handle = "@" + handle
	}

	hm, err := loadHandles()
	if err != nil {
		return 0, err
	}

	nodeID, ok := hm.Handles[handle]
	if !ok {
		return 0, fmt.Errorf("handle %s not found. Run `agios browser page` to refresh handles", handle)
	}

	if nodeID == 0 {
		return 0, fmt.Errorf("handle %s has no associated DOM node", handle)
	}

	return nodeID, nil
}
