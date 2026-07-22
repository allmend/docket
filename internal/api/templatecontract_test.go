package api

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/template/parse"
)

// This file enforces the contract between a template and the handler that
// renders it, statically — no fixtures to maintain.
//
// The drift it catches: a template starts referencing a new field, one handler
// is updated to supply it, and the other handlers that render the same template
// are forgotten. html/template renders the missing key as empty, so the page
// looks fine and the omission ships. That has happened repeatedly here; see the
// CLAUDE.md entries on partials not "following along".
//
// Both sides are derived mechanically:
//   - required fields come from walking the template parse tree
//   - supplied keys come from the map literal at each h.render call site
//
// This pairs with missingkey=error in parseTemplates: that turns a miss into a
// runtime error, this turns it into a failing build.

// ---------- template side: which root fields does a template require? ----------

// rootFields returns the top-level data fields the entry template needs,
// following {{template}} includes that pass the root context through.
func rootFields(set *template.Template, entry string) map[string]bool {
	out := map[string]bool{}
	visited := map[string]bool{}

	var walk func(name string, dotIsRoot bool)
	var node func(n parse.Node, dotIsRoot bool)

	// pipe records root-level field references inside a pipeline.
	var pipe func(p *parse.PipeNode, dotIsRoot bool)
	pipe = func(p *parse.PipeNode, dotIsRoot bool) {
		if p == nil {
			return
		}
		for _, cmd := range p.Cmds {
			for _, arg := range cmd.Args {
				switch a := arg.(type) {
				case *parse.FieldNode:
					// {{.Foo}} / {{.Foo.Bar}} — only the first segment is a data key.
					if dotIsRoot && len(a.Ident) > 0 {
						out[a.Ident[0]] = true
					}
				case *parse.VariableNode:
					// {{$.Foo}} always refers to the root, whatever the current dot.
					if len(a.Ident) > 1 && a.Ident[0] == "$" {
						out[a.Ident[1]] = true
					}
				case *parse.ChainNode:
					if _, ok := a.Node.(*parse.DotNode); ok && dotIsRoot && len(a.Field) > 0 {
						out[a.Field[0]] = true
					}
				case *parse.PipeNode:
					pipe(a, dotIsRoot)
				}
			}
		}
	}

	node = func(n parse.Node, dotIsRoot bool) {
		switch t := n.(type) {
		case *parse.ListNode:
			if t == nil {
				return
			}
			for _, c := range t.Nodes {
				node(c, dotIsRoot)
			}
		case *parse.ActionNode:
			pipe(t.Pipe, dotIsRoot)
		case *parse.IfNode:
			// {{if}} does not rebind dot.
			pipe(t.Pipe, dotIsRoot)
			node(t.List, dotIsRoot)
			node(t.ElseList, dotIsRoot)
		case *parse.WithNode:
			// {{with}} rebinds dot inside the body, not in the else branch.
			pipe(t.Pipe, dotIsRoot)
			node(t.List, false)
			node(t.ElseList, dotIsRoot)
		case *parse.RangeNode:
			pipe(t.Pipe, dotIsRoot)
			node(t.List, false)
			node(t.ElseList, dotIsRoot)
		case *parse.TemplateNode:
			// {{template "x"}} and {{template "x" .}} keep the current context;
			// {{template "x" (dict ...)}} builds a fresh one, so its fields are
			// not our caller's responsibility.
			passesRoot := t.Pipe == nil || isDotPipe(t.Pipe)
			pipe(t.Pipe, dotIsRoot)
			if passesRoot {
				walk(t.Name, dotIsRoot)
			}
		}
	}

	walk = func(name string, dotIsRoot bool) {
		key := name + fmt.Sprint(dotIsRoot)
		if visited[key] {
			return
		}
		visited[key] = true
		t := set.Lookup(name)
		if t == nil || t.Tree == nil {
			return
		}
		node(t.Tree.Root, dotIsRoot)
	}

	walk(entry, true)
	return out
}

// isDotPipe reports whether a pipeline is exactly ".".
func isDotPipe(p *parse.PipeNode) bool {
	if p == nil || len(p.Cmds) != 1 || len(p.Cmds[0].Args) != 1 {
		return false
	}
	_, ok := p.Cmds[0].Args[0].(*parse.DotNode)
	return ok
}

// ---------- handler side: which keys does each render call supply? ----------

type renderSite struct {
	template string
	keys     map[string]bool
	pos      string
	analysed bool // false when the argument isn't a map literal we can read
}

// pageDataKeys are the nav keys h.pageData injects on top of the caller's map.
// pageData must set every one of these unconditionally — see TestPageDataSeedsAllKeys.
var pageDataKeys = []string{
	"NavTeams", "CurrentUser", "NavUnreadCount", "NavMyIssueCount",
	"OrgName", "OrgSlug", "CurrentPath", "NavTeam", "NavBoard",
	"NavTags", "NavActiveSprint",
}

func collectRenderSites(t *testing.T) []renderSite {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse api package: %v", err)
	}

	// Index package-level funcs so a render site passing h.someHelper(...) can be
	// resolved to the map that helper builds — that's how the board, planning and
	// backlog pages get their data, and they're the ones we least want to break.
	helpers := map[string]*ast.FuncDecl{}
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			for _, d := range file.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok {
					helpers[fn.Name.Name] = fn
				}
			}
		}
	}

	var sites []renderSite
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			if strings.HasSuffix(fname, "_test.go") {
				continue
			}
			for _, d := range file.Decls {
				fn, isFn := d.(*ast.FuncDecl)
				if !isFn || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "render" || len(call.Args) != 3 {
						return true
					}
					lit, ok := call.Args[1].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					name, err := strconv.Unquote(lit.Value)
					if err != nil {
						return true
					}
					keys, viaPageData, ok := resolveData(call.Args[2], fn, helpers, 0)
					site := renderSite{
						template: name,
						keys:     keys,
						pos:      filepath.Base(fset.Position(call.Pos()).String()),
						analysed: ok,
					}
					if viaPageData {
						for _, k := range pageDataKeys {
							site.keys[k] = true
						}
					}
					sites = append(sites, site)
					return true
				})
			}
		}
	}
	return sites
}

// resolveData works out which keys a render call's data argument carries. It
// handles map literals, h.pageData(...) wrappers, local `data := map[...]{...}`
// variables (including later data["K"] = v assignments), and calls to package
// helpers that return a map.
func resolveData(e ast.Expr, scope *ast.FuncDecl, helpers map[string]*ast.FuncDecl, depth int) (map[string]bool, bool, bool) {
	if depth > 4 {
		return map[string]bool{}, false, false
	}
	switch v := e.(type) {
	case *ast.Ident:
		if v.Name == "nil" {
			return map[string]bool{}, false, true
		}
		// A local variable — collect its literal and any later index assignments.
		if scope == nil {
			return map[string]bool{}, false, false
		}
		keys, viaPageData, found := map[string]bool{}, false, false
		ast.Inspect(scope.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, lhs := range as.Lhs {
				switch l := lhs.(type) {
				case *ast.Ident:
					if l.Name == v.Name && i < len(as.Rhs) {
						if k, p, o := resolveData(as.Rhs[i], scope, helpers, depth+1); o {
							for key := range k {
								keys[key] = true
							}
							viaPageData = viaPageData || p
							found = true
						}
					}
				case *ast.IndexExpr:
					// data["Key"] = ...
					id, isID := l.X.(*ast.Ident)
					lit, isLit := l.Index.(*ast.BasicLit)
					if isID && id.Name == v.Name && isLit && lit.Kind == token.STRING {
						if s, err := strconv.Unquote(lit.Value); err == nil {
							keys[s] = true
						}
					}
				}
			}
			return true
		})
		return keys, viaPageData, found

	case *ast.CallExpr:
		sel, isSel := v.Fun.(*ast.SelectorExpr)
		if !isSel {
			return map[string]bool{}, false, false
		}
		if sel.Sel.Name == "pageData" && len(v.Args) == 2 {
			inner, _, ok := resolveData(v.Args[1], scope, helpers, depth+1)
			return inner, true, ok
		}
		// A helper that returns a map: analyse its return statement.
		fn, ok := helpers[sel.Sel.Name]
		if !ok || fn.Body == nil {
			return map[string]bool{}, false, false
		}
		keys, viaPageData, found := map[string]bool{}, false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ret, isRet := n.(*ast.ReturnStmt)
			if !isRet || len(ret.Results) != 1 {
				return true
			}
			if k, p, o := resolveData(ret.Results[0], fn, helpers, depth+1); o {
				for key := range k {
					keys[key] = true
				}
				viaPageData = viaPageData || p
				found = true
			}
			return true
		})
		return keys, viaPageData, found
	}
	return mapKeys(e)
}

// mapKeys reads the keys out of a map[string]any composite literal, unwrapping
// an h.pageData(r, …) call if present.
func mapKeys(e ast.Expr) (keys map[string]bool, viaPageData bool, ok bool) {
	keys = map[string]bool{}
	switch v := e.(type) {
	case *ast.CallExpr:
		sel, isSel := v.Fun.(*ast.SelectorExpr)
		if isSel && sel.Sel.Name == "pageData" && len(v.Args) == 2 {
			inner, _, innerOK := mapKeys(v.Args[1])
			return inner, true, innerOK
		}
		return keys, false, false
	case *ast.CompositeLit:
		for _, elt := range v.Elts {
			kv, isKV := elt.(*ast.KeyValueExpr)
			if !isKV {
				continue
			}
			lit, isLit := kv.Key.(*ast.BasicLit)
			if !isLit || lit.Kind != token.STRING {
				continue
			}
			k, err := strconv.Unquote(lit.Value)
			if err == nil {
				keys[k] = true
			}
		}
		return keys, false, true
	case *ast.Ident:
		if v.Name == "nil" {
			return keys, false, true // nil data supplies nothing
		}
		return keys, false, false
	}
	return keys, false, false
}

// ---------- the test ----------

// TestTemplateDataContract fails when a handler renders a template that
// references data the handler does not pass.
func TestTemplateDataContract(t *testing.T) {
	tmpls, err := parseTemplates("../../templates")
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}

	sites := collectRenderSites(t)
	if len(sites) == 0 {
		t.Fatal("found no h.render call sites — the AST scan is broken")
	}

	var skipped []string
	for _, s := range sites {
		set := tmpls[s.template]
		if set == nil {
			t.Errorf("%s renders %q, which is not in the parsed template set", s.pos, s.template)
			continue
		}
		if !s.analysed {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", s.pos, s.template))
			continue
		}

		entry := "base"
		if set.Lookup("base") == nil {
			entry = strings.TrimSuffix(s.template, filepath.Ext(s.template))
		}

		var missing []string
		for f := range rootFields(set, entry) {
			if !s.keys[f] {
				missing = append(missing, f)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("%s renders %q without: %s",
				s.pos, s.template, strings.Join(missing, ", "))
		}
	}

	// Non-literal data (structs, variables) can't be checked this way; report the
	// coverage gap rather than pretending it's clean.
	if len(skipped) > 0 {
		sort.Strings(skipped)
		t.Logf("%d render sites not statically checkable (non-map data): %s",
			len(skipped), strings.Join(skipped, ", "))
	}
}

// TestPageDataSeedsAllKeys pins the base-layout contract: pageData must set every
// nav key on every path. They used to be set only on success (NavTags and
// NavActiveSprint only when the page passed a Board), which under missingkey=error
// turns any org-level page — or one failed lookup — into a 500 rather than a
// slightly emptier sidebar.
func TestPageDataSeedsAllKeys(t *testing.T) {
	src, err := parser.ParseFile(token.NewFileSet(), "handler.go", nil, 0)
	if err != nil {
		t.Fatalf("parse handler.go: %v", err)
	}

	var body string
	ast.Inspect(src, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "pageData" || fn.Body == nil {
			return true
		}
		body = fmt.Sprint(fn.Body.Pos(), fn.Body.End())
		return false
	})
	if body == "" {
		t.Fatal("pageData not found in handler.go")
	}

	// Every key must appear in the unconditional seed block or as a direct assign.
	raw, err := readFile("handler.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range pageDataKeys {
		if k == "CurrentPath" {
			continue // always assigned directly
		}
		if !strings.Contains(raw, `"`+k+`"`) {
			t.Errorf("pageData no longer mentions %q — the base layout still reads it", k)
		}
	}
}

// readFile is a tiny helper so the contract tests can grep source text.
func readFile(name string) (string, error) {
	b, err := os.ReadFile(name)
	return string(b), err
}

// TestPartialDictContract checks the other half of the contract: partials invoked
// as {{template "x" (dict "K" v …)}} receive a map, so under missingkey=error any
// field the partial reads but the dict omits is a render error. There are 33 such
// call sites, and they are the most common way a partial gets "forgotten" when it
// grows a new field.
func TestPartialDictContract(t *testing.T) {
	tmpls, err := parseTemplates("../../templates")
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}

	type call struct {
		caller, callee string
		keys           map[string]bool
	}
	var calls []call
	seen := map[string]bool{}

	// Walk every parsed set. Dedupe per (set, template name), not by name alone:
	// every page defines a template literally called "content", so a global
	// name-dedupe silently skips every page after the first.
	var set *template.Template
	type namedTmpl struct {
		setKey string
		tp     *template.Template
	}
	var templatesToWalk []namedTmpl
	for key, s := range tmpls {
		if set == nil {
			set = s
		}
		for _, tp := range s.Templates() {
			templatesToWalk = append(templatesToWalk, namedTmpl{key, tp})
		}
	}
	if set == nil {
		t.Fatal("no templates parsed")
	}

	for _, nt := range templatesToWalk {
		tp := nt.tp
		dedupe := nt.setKey + "::" + tp.Name()
		if tp.Tree == nil || seen[dedupe] {
			continue
		}
		seen[dedupe] = true
		caller := tp.Name()

		var walk func(n parse.Node)
		walk = func(n parse.Node) {
			switch v := n.(type) {
			case *parse.ListNode:
				if v == nil {
					return
				}
				for _, c := range v.Nodes {
					walk(c)
				}
			case *parse.IfNode:
				walk(v.List)
				walk(v.ElseList)
			case *parse.WithNode:
				walk(v.List)
				walk(v.ElseList)
			case *parse.RangeNode:
				walk(v.List)
				walk(v.ElseList)
			case *parse.TemplateNode:
				if keys, ok := dictKeys(v.Pipe); ok {
					calls = append(calls, call{caller: caller, callee: v.Name, keys: keys})
				}
			}
		}
		walk(tp.Tree.Root)
	}

	if len(calls) == 0 {
		t.Fatal("found no dict-based template calls — the walker is broken")
	}

	reported := map[string]bool{}
	for _, c := range calls {
		var missing []string
		for f := range rootFields(set, c.callee) {
			if !c.keys[f] {
				missing = append(missing, f)
			}
		}
		sort.Strings(missing)
		sig := c.callee + "|" + strings.Join(missing, ",")
		if len(missing) > 0 && !reported[sig] {
			reported[sig] = true
			t.Errorf("%s calls %q with a dict missing: %s",
				c.caller, c.callee, strings.Join(missing, ", "))
		}
	}
	t.Logf("checked %d dict-based partial calls", len(calls))
}

// dictKeys extracts the literal keys from a `(dict "K" v "K2" v2)` pipeline.
// Reports false when the pipeline isn't a dict call.
func dictKeys(p *parse.PipeNode) (map[string]bool, bool) {
	if p == nil || len(p.Cmds) != 1 {
		return nil, false
	}
	args := p.Cmds[0].Args
	if len(args) < 1 {
		return nil, false
	}
	// `(dict …)` parses as a parenthesised sub-pipeline, so unwrap one level.
	if inner, ok := args[0].(*parse.PipeNode); ok && len(args) == 1 {
		return dictKeys(inner)
	}
	id, ok := args[0].(*parse.IdentifierNode)
	if !ok || id.Ident != "dict" {
		return nil, false
	}
	keys := map[string]bool{}
	for i := 1; i < len(args); i += 2 {
		if s, ok := args[i].(*parse.StringNode); ok {
			keys[s.Text] = true
		}
	}
	return keys, true
}
