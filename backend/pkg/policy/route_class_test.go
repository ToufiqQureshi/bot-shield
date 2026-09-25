package policy

import "testing"

func TestClassifyRouteOnlyUsesActivatedExactTenantLabels(t *testing.T) {
	p := &Policy{Mode: "shadow", RouteClasses: map[string]string{"/account/signin": ClassLogin}}
	if got := p.ClassifyRoute("/account/signin", "POST"); got != ClassBrowse {
		t.Fatalf("shadow draft changed live class to %q", got)
	}
	p.Mode = "enforce"
	if got := p.ClassifyRoute("/account/signin", "POST"); got != ClassLogin {
		t.Fatalf("activated custom login=%q", got)
	}
	if got := p.ClassifyRoute("/account/signin/extra", "POST"); got != ClassBrowse {
		t.Fatalf("non-exact route=%q", got)
	}
	if got := p.ClassifyRoute("/LOGIN", "POST"); got != ClassLogin {
		t.Fatalf("default login lost classification: %q", got)
	}
	p.RouteClasses["/login"] = ClassCheckout // defense against a malformed cached policy
	if got := p.ClassifyRoute("/login", "POST"); got != ClassLogin {
		t.Fatalf("malformed policy loosened built-in login to %q", got)
	}
}

func TestCompiledPolicyOwnsRouteLabels(t *testing.T) {
	routes := map[string]string{"/account/signin": ClassLogin}
	p, err := Compile(&Policy{Mode: "enforce", RouteClasses: routes})
	if err != nil {
		t.Fatal(err)
	}
	routes["/account/signin"] = ClassCheckout
	if got := p.ClassifyRoute("/account/signin", "POST"); got != ClassLogin {
		t.Fatalf("mutable source map changed compiled policy: %q", got)
	}
}
