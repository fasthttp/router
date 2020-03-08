package radix

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

// Used as a workaround since we can't compare functions or their addresses
var fakeHandlerValue string

func fakeHandler(val string) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		fakeHandlerValue = val
	}
}

type testRequests []struct {
	path       string
	nilHandler bool
	route      string
	ps         map[string]interface{}
}

func acquireRequestCtx(path string) *fasthttp.RequestCtx {
	var requestCtx fasthttp.RequestCtx
	var fastRequest fasthttp.Request
	fastRequest.SetRequestURI(path)
	requestCtx.Init(&fastRequest, nil, nil)
	return &requestCtx
}

func checkRequests(t *testing.T, tree *Tree, requests testRequests) {
	for _, request := range requests {
		ctx := acquireRequestCtx(request.path)
		handler, _ := tree.Get(request.path, ctx)

		if handler == nil {
			if !request.nilHandler {
				t.Errorf("handle mismatch for route '%s': Expected non-nil handle", request.path)
			}
		} else if request.nilHandler {
			t.Errorf("handle mismatch for route '%s': Expected nil handle", request.path)
		} else {
			handler(ctx)
			if fakeHandlerValue != request.route {
				t.Errorf("handle mismatch for route '%s': Wrong handle (%s != %s)", request.path, fakeHandlerValue, request.route)
			}
		}

		params := make(map[string]interface{})
		if request.ps == nil {
			request.ps = make(map[string]interface{})
		}

		ctx.VisitUserValues(func(key []byte, value interface{}) {
			params[string(key)] = value
		})

		if !reflect.DeepEqual(params, request.ps) {
			t.Errorf("Route %s - User values == %v, want %v", request.path, params, request.ps)
		}
	}
}

func TestTreeAddAndGet(t *testing.T) {
	tree := New()

	routes := [...]string{
		"/hi",
		"/contact",
		"/co",
		"/c",
		"/a",
		"/ab",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/α",
		"/β",
	}
	for _, route := range routes {
		tree.Add(route, fakeHandler(route))
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/a", false, "/a", nil},
		{"/", true, "", nil},
		{"/hi", false, "/hi", nil},
		{"/contact", false, "/contact", nil},
		{"/co", false, "/co", nil},
		{"/con", true, "", nil},  // key mismatch
		{"/cona", true, "", nil}, // key mismatch
		{"/no", true, "", nil},   // no matching child
		{"/ab", false, "/ab", nil},
		{"/α", false, "/α", nil},
		{"/β", false, "/β", nil},
	})
}

func TestTreeWildcard(t *testing.T) {
	tree := New()

	routes := [...]string{
		"/",
		"/cmd/:tool/:sub",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/",
		"/search/:query",
		"/user_:name",
		"/user_:name/about",
		"/files/:dir/*filepath",
		"/doc/",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/info/:user/public",
		"/info/:user/project/:project",
	}
	for _, route := range routes {
		tree.Add(route, fakeHandler(route))
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/cmd/test/", false, "/cmd/:tool/", map[string]interface{}{"tool": "test"}},
		{"/cmd/test", true, "", map[string]interface{}{"tool": "test"}},
		{"/cmd/test/3", false, "/cmd/:tool/:sub", map[string]interface{}{"tool": "test", "sub": "3"}},
		{"/src/", false, "/src/*filepath", map[string]interface{}{"filepath": "/"}},
		{"/src/some/file.png", false, "/src/*filepath", map[string]interface{}{"filepath": "some/file.png"}},
		{"/search/", false, "/search/", nil},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", map[string]interface{}{"query": "someth!ng+in+ünìcodé"}},
		{"/search/someth!ng+in+ünìcodé/", true, "", map[string]interface{}{"query": "someth!ng+in+ünìcodé"}},
		{"/user_gopher", false, "/user_:name", map[string]interface{}{"name": "gopher"}},
		{"/user_gopher/about", false, "/user_:name/about", map[string]interface{}{"name": "gopher"}},
		{"/files/js/inc/framework.js", false, "/files/:dir/*filepath", map[string]interface{}{"dir": "js", "filepath": "inc/framework.js"}},
		{"/info/gordon/public", false, "/info/:user/public", map[string]interface{}{"user": "gordon"}},
		{"/info/gordon/project/go", false, "/info/:user/project/:project", map[string]interface{}{"user": "gordon", "project": "go"}},
	})
}

func catchPanic(testFunc func()) (recv interface{}) {
	defer func() {
		recv = recover()
	}()

	testFunc()
	return
}

type testRoute struct {
	path     string
	conflict bool
}

func testRoutes(t *testing.T, routes []testRoute) {
	tree := New()

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.Add(route.path, nil)
		})

		if route.conflict {
			if recv == nil {
				t.Errorf("no panic for conflicting route '%s'", route.path)
			}
		} else if recv != nil {
			t.Errorf("unexpected panic for route '%s': %v", route.path, recv)
		}
	}

	//printChildren(tree, "")
}

func TestTreeWildcardConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/:tool/:sub", false},
		{"/cmd/vet", false},
		{"/src/*filepath", false},
		{"/src/*filepathx", true},
		{"/src/", false},
		{"/src1/", false},
		{"/src1/*filepath", false},
		{"/src2*filepath", true},
		{"/search/:query", false},
		{"/search/invalid", false},
		{"/user_:name", false},
		{"/user_x", false},
		{"/user_:name", true},
		{"/id:id", false},
		{"/id/:id", false},
	}
	testRoutes(t, routes)
}

func TestTreeChildConflict(t *testing.T) {
	routes := []testRoute{
		{"/cmd/vet", false},
		{"/cmd/:tool/:sub", false},
		{"/src/AUTHORS", false},
		{"/src/*filepath", false},
		{"/user_x", false},
		{"/user_:name", false},
		{"/id/:id", false},
		{"/id:id", false},
		{"/:users", false},
		{"/:id/", true},
		{"/*filepath", false},
		{"/asd*filepath", true},
	}
	testRoutes(t, routes)
}

func TestTreeDuplicatePath(t *testing.T) {
	tree := New()

	routes := [...]string{
		"/",
		"/doc/",
		"/src/*filepath",
		"/search/:query",
		"/user_:name",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.Add(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}

		// Add again
		recv = catchPanic(func() {
			tree.Add(route, nil)
		})
		if recv == nil {
			t.Fatalf("no panic while inserting duplicate route '%s", route)
		}
	}

	//printChildren(tree, "")

	checkRequests(t, tree, testRequests{
		{"/", false, "/", nil},
		{"/doc/", false, "/doc/", nil},
		{"/src/some/file.png", false, "/src/*filepath", map[string]interface{}{"filepath": "some/file.png"}},
		{"/search/someth!ng+in+ünìcodé", false, "/search/:query", map[string]interface{}{"query": "someth!ng+in+ünìcodé"}},
		{"/user_gopher", false, "/user_:name", map[string]interface{}{"name": "gopher"}},
	})
}

func TestEmptyWildcardName(t *testing.T) {
	tree := New()

	routes := [...]string{
		"/user:",
		"/user:/",
		"/cmd/:/",
		"/src/*",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.Add(route, nil)
		})
		if recv == nil {
			t.Errorf("no panic while inserting route with empty wildcard name '%s", route)
		}
	}
}

func TestTreeCatchAllConflict(t *testing.T) {
	routes := []testRoute{
		{"/src/*filepath/x", true},
		{"/src2/", false},
		{"/src2/*filepath/x", true},
		{"/src3/*filepath", false},
		{"/src3/*filepath/x", true},
	}
	testRoutes(t, routes)
}

func TestTreeCatchAllConflictRoot(t *testing.T) {
	routes := []testRoute{
		{"/", false},
		{"/*filepath", false},
	}
	testRoutes(t, routes)
}

func TestTreeCatchMaxParams(t *testing.T) {
	tree := New()
	var route = "/cmd/*filepath"
	tree.Add(route, fakeHandler(route))
}

func TestTreeDoubleWildcard(t *testing.T) {
	const panicMsg = "only one wildcard per path segment is allowed"

	routes := [...]string{
		"/:foo:bar",
		"/:foo:bar/",
		"/:foo*bar",
	}

	for _, route := range routes {
		tree := New()
		recv := catchPanic(func() {
			tree.Add(route, nil)
		})

		if rs, ok := recv.(string); !ok || !strings.HasPrefix(rs, panicMsg) {
			t.Fatalf(`"Expected panic "%s" for route '%s', got "%v"`, panicMsg, route, recv)
		}
	}
}

/*func TestTreeDuplicateWildcard(t *testing.T) {
	tree := &node{}

	routes := [...]string{
		"/:id/:name/:id",
	}
	for _, route := range routes {
		...
	}
}*/

func TestTreeTrailingSlashRedirect(t *testing.T) {
	tree := New()

	routes := [...]string{
		"/hi",
		"/b/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/admin",
		"/admin/:category",
		"/admin/:category/:page",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/no/a",
		"/no/b",
		"/api/hello/:name",
	}
	for _, route := range routes {
		recv := catchPanic(func() {
			tree.Add(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	//printChildren(tree, "")

	tsrRoutes := [...]string{
		"/hi/",
		"/b",
		"/search/gopher/",
		"/cmd/vet",
		"/src",
		"/x/",
		"/y",
		"/0/go/",
		"/1/go",
		"/a",
		"/admin/",
		"/admin/config/",
		"/admin/config/permissions/",
		"/doc/",
	}
	for _, route := range tsrRoutes {
		handler, tsr := tree.Get(route, nil)
		if handler != nil {
			t.Fatalf("non-nil handler for TSR route '%s", route)
		} else if !tsr {
			t.Errorf("expected TSR recommendation for route '%s'", route)
		}
	}

	noTsrRoutes := [...]string{
		"/",
		"/no",
		"/no/",
		"/_",
		"/_/",
		"/api/world/abc",
	}
	for _, route := range noTsrRoutes {
		handler, tsr := tree.Get(route, nil)
		if handler != nil {
			t.Fatalf("non-nil handler for No-TSR route '%s", route)
		} else if tsr {
			t.Errorf("expected no TSR recommendation for route '%s'", route)
		}
	}
}

func TestTreeRootTrailingSlashRedirect(t *testing.T) {
	tree := New()

	recv := catchPanic(func() {
		tree.Add("/:test", fakeHandler("/:test"))
	})
	if recv != nil {
		t.Fatalf("panic inserting test route: %v", recv)
	}

	handler, tsr := tree.Get("/", nil)
	if handler != nil {
		t.Fatalf("non-nil handler")
	} else if tsr {
		t.Errorf("expected no TSR recommendation")
	}
}

func TestTreeFindCaseInsensitivePath(t *testing.T) {
	tree := New()

	longPath := "/l" + strings.Repeat("o", 128) + "ng"
	lOngPath := "/l" + strings.Repeat("O", 128) + "ng/"

	routes := [...]string{
		"/hi",
		"/b/",
		"/ABC/",
		"/search/:query",
		"/cmd/:tool/",
		"/src/*filepath",
		"/x",
		"/x/y",
		"/y/",
		"/y/z",
		"/0/:id",
		"/0/:id/1",
		"/1/:id/",
		"/1/:id/2",
		"/aa",
		"/a/",
		"/doc",
		"/doc/go_faq.html",
		"/doc/go1.html",
		"/doc/go/away",
		"/no/a",
		"/no/b",
		"/Π",
		"/u/apfêl/",
		"/u/äpfêl/",
		"/u/äpkul/",
		"/u/öpfêl",
		"/v/Äpfêl/",
		"/v/Öpfêl",
		"/w/♬",  // 3 byte
		"/w/♭/", // 3 byte, last byte differs
		"/w/𠜎",  // 4 byte
		"/w/𠜏/", // 4 byte
		longPath,
	}

	for _, route := range routes {
		recv := catchPanic(func() {
			tree.Add(route, fakeHandler(route))
		})
		if recv != nil {
			t.Fatalf("panic inserting route '%s': %v", route, recv)
		}
	}

	// Check out == in for all registered routes
	// With fixTrailingSlash = true
	for _, route := range routes {
		out, found := tree.FindCaseInsensitivePath(route, true)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}
	// With fixTrailingSlash = false
	for _, route := range routes {
		out, found := tree.FindCaseInsensitivePath(route, false)
		if !found {
			t.Errorf("Route '%s' not found!", route)
		} else if string(out) != route {
			t.Errorf("Wrong result for route '%s': %s", route, string(out))
		}
	}

	tests := []struct {
		in    string
		out   string
		found bool
		slash bool
	}{
		{"/HI", "/hi", true, false},
		{"/HI/", "/hi", true, true},
		{"/B", "/b/", true, true},
		{"/B/", "/b/", true, false},
		{"/abc", "/ABC/", true, true},
		{"/abc/", "/ABC/", true, false},
		{"/aBc", "/ABC/", true, true},
		{"/aBc/", "/ABC/", true, false},
		{"/abC", "/ABC/", true, true},
		{"/abC/", "/ABC/", true, false},
		{"/SEARCH/QUERY", "/search/QUERY", true, false},
		{"/SEARCH/QUERY/", "/search/QUERY", true, true},
		{"/CMD/TOOL/", "/cmd/TOOL/", true, false},
		{"/CMD/TOOL", "/cmd/TOOL/", true, true},
		{"/SRC/FILE/PATH", "/src/FILE/PATH", true, false},
		{"/x/Y", "/x/y", true, false},
		{"/x/Y/", "/x/y", true, true},
		{"/X/y", "/x/y", true, false},
		{"/X/y/", "/x/y", true, true},
		{"/X/Y", "/x/y", true, false},
		{"/X/Y/", "/x/y", true, true},
		{"/Y/", "/y/", true, false},
		{"/Y", "/y/", true, true},
		{"/Y/z", "/y/z", true, false},
		{"/Y/z/", "/y/z", true, true},
		{"/Y/Z", "/y/z", true, false},
		{"/Y/Z/", "/y/z", true, true},
		{"/y/Z", "/y/z", true, false},
		{"/y/Z/", "/y/z", true, true},
		{"/Aa", "/aa", true, false},
		{"/Aa/", "/aa", true, true},
		{"/AA", "/aa", true, false},
		{"/AA/", "/aa", true, true},
		{"/aA", "/aa", true, false},
		{"/aA/", "/aa", true, true},
		{"/A/", "/a/", true, false},
		{"/A", "/a/", true, true},
		{"/DOC", "/doc", true, false},
		{"/DOC/", "/doc", true, true},
		{"/NO", "", false, true},
		{"/DOC/GO", "", false, true},
		{"/π", "/Π", true, false},
		{"/π/", "/Π", true, true},
		{"/u/ÄPFÊL/", "/u/äpfêl/", true, false},
		{"/U/ÄPKUL/", "/u/äpkul/", true, false},
		{"/u/ÄPFÊL", "/u/äpfêl/", true, true},
		{"/u/ÖPFÊL/", "/u/öpfêl", true, true},
		{"/u/ÖPFÊL", "/u/öpfêl", true, false},
		{"/v/äpfêL/", "/v/Äpfêl/", true, false},
		{"/v/äpfêL", "/v/Äpfêl/", true, true},
		{"/v/öpfêL/", "/v/Öpfêl", true, true},
		{"/v/öpfêL", "/v/Öpfêl", true, false},
		{"/w/♬/", "/w/♬", true, true},
		{"/w/♭", "/w/♭/", true, true},
		{"/w/𠜎/", "/w/𠜎", true, true},
		{"/w/𠜏", "/w/𠜏/", true, true},
		{lOngPath, longPath, true, true},
	}
	// With fixTrailingSlash = true
	for _, test := range tests {
		out, found := tree.FindCaseInsensitivePath(test.in, true)
		if found != test.found || (found && (string(out) != test.out)) {
			t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
				test.in, string(out), found, test.out, test.found)
		}
	}
	// With fixTrailingSlash = false
	for _, test := range tests {
		out, found := tree.FindCaseInsensitivePath(test.in, false)
		if test.slash {
			if found { // test needs a trailingSlash fix. It must not be found!
				t.Errorf("Found without fixTrailingSlash: %s; got %s", test.in, string(out))
			}
		} else {
			if found != test.found || (found && (string(out) != test.out)) {
				t.Errorf("Wrong result for '%s': got %s, %t; want %s, %t",
					test.in, string(out), found, test.out, test.found)
			}
		}
	}
}

func TestTreeInvalidNodeType(t *testing.T) {
	const panicMsg = "invalid node type"

	tree := New()
	tree.Add("/", fakeHandler("/"))
	tree.Add("/:page", fakeHandler("/:page"))

	// set invalid node type
	tree.root.children[0].nType = 42

	// normal lookup
	recv := catchPanic(func() {
		tree.Get("/test", nil)
	})
	if rs, ok := recv.(string); !ok || rs != panicMsg {
		t.Fatalf("Expected panic '"+panicMsg+"', got '%v'", recv)
	}

	// case-insensitive lookup
	recv = catchPanic(func() {
		tree.FindCaseInsensitivePath("/test", true)
	})
	if rs, ok := recv.(string); !ok || rs != panicMsg {
		t.Fatalf("Expected panic '"+panicMsg+"', got '%v'", recv)
	}
}

func TestTreeWildcardConflictEx(t *testing.T) {
	conflicts := []struct {
		route string

		wantErr     bool
		wantErrText string
	}{
		{route: "/who/are/foo", wantErr: false},
		{route: "/who/are/foo/", wantErr: false},
		{route: "/who/are/foo/bar", wantErr: false},
		{route: "/conxxx", wantErr: false},
		{route: "/conooo/xxx", wantErr: false},
		{
			route:       "invalid/data",
			wantErr:     true,
			wantErrText: "path must begin with '/' in path 'invalid/data'",
		},
		{
			route:       "/con:tact",
			wantErr:     true,
			wantErrText: "':tact' in new path '/con:tact' conflicts with existing wildcard ':tact' in existing prefix '/con:tact'",
		},
		{
			route:       "/who/are/*me",
			wantErr:     true,
			wantErrText: "'*me' in new path '/who/are/*me' conflicts with existing wildcard '*you' in existing prefix '/who/are/*you'",
		},
		{
			route:       "/who/foo/hello",
			wantErr:     true,
			wantErrText: "a handle is already registered for path '/who/foo/hello'",
		},
		{
			route:       "/*static",
			wantErr:     true,
			wantErrText: "'*static' in new path '/*static' conflicts with existing wildcard '*filepath' in existing prefix '/*filepath'",
		},
		{
			route:       "/static/*filepath/other",
			wantErr:     true,
			wantErrText: "wildcard routes are only allowed at the end of the path in path '/static/*filepath/other'",
		},
		{
			route:       "/:user/",
			wantErr:     true,
			wantErrText: "':user' in new path '/:user/' conflicts with existing wildcard ':id' in existing prefix '/:id'",
		},
		{
			route:       "/prefix*filepath",
			wantErr:     true,
			wantErrText: "no / before wildcard in path '/prefix*filepath'",
		},
	}

	for _, conflict := range conflicts {
		// I have to re-create a 'tree', because the 'tree' will be
		// in an inconsistent state when the loop recovers from the
		// panic which threw by 'addRoute' function.
		tree := New()
		routes := [...]string{
			"/con:tact",
			"/who/are/*you",
			"/who/foo/hello",
			"/*filepath",
			"/:id",
		}

		for _, route := range routes {
			tree.Add(route, fakeHandler(route))
		}

		err := catchPanic(func() {
			tree.Add(conflict.route, fakeHandler(conflict.route))
		})

		if conflict.wantErr == (err == nil) {
			t.Errorf("Unexpected error: %v", err)
		}

		if err != nil && conflict.wantErrText != fmt.Sprint(err) {
			t.Errorf("Invalid conflict error text (%v)", err)
		}

	}
}
