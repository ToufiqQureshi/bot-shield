package policy

import "testing"

func TestNormalizeAndClassify(t *testing.T) {
	cases := []struct {
		input, method, wantPath, wantClass string
		ok                                 bool
	}{
		{"/login", "POST", "/login", ClassLogin, true},
		{"/api/v1/items", "GET", "/api/v1/items", ClassAPI, true},
		{"/checkout/pay", "POST", "/checkout/pay", ClassCheckout, true},
		{"/assets/app.js", "GET", "/assets/app.js", ClassStatic, true},
		{"/shop/../pricing", "GET", "/pricing", ClassBrowse, true},
		{"/a%252fb", "GET", "", "", false},
		{"/a\\b", "GET", "", "", false},
		{"/a//b", "GET", "", "", false},
	}
	for _, tc := range cases {
		path, ok := NormalizePath(tc.input)
		if ok != tc.ok || path != tc.wantPath {
			t.Errorf("NormalizePath(%q)=(%q,%v), want (%q,%v)", tc.input, path, ok, tc.wantPath, tc.ok)
		}
		if ok && Classify(path, tc.method) != tc.wantClass {
			t.Errorf("Classify(%q,%q)=%q, want %q", path, tc.method, Classify(path, tc.method), tc.wantClass)
		}
	}
}

func TestPhase1ConditionsAndCompile(t *testing.T) {
	rule := Rule{ID: "r1", Name: "login bot", Enabled: true, Action: ActionBlock, Conditions: []Condition{
		{Field: FieldRequestClass, Operator: OpEquals, Value: ClassLogin},
		{Field: FieldSignal, Operator: OpEquals, Value: "velocity_spike"},
		{Field: FieldPath, Operator: OpMatches, Value: `^/login$`},
	}}
	if err := ValidateRule(rule, 100); err != nil {
		t.Fatal(err)
	}
	p, err := Compile(&Policy{Rules: []Rule{rule}})
	if err != nil {
		t.Fatal(err)
	}
	if !Evaluate(p, Facts{Path: "/login", Class: ClassLogin, Signals: []string{"velocity_spike"}}).Matched {
		t.Fatal("valid phase1 rule did not match")
	}
	if Evaluate(p, Facts{Path: "/login", Class: ClassLogin, Signals: []string{"header_anomaly"}}).Matched {
		t.Fatal("wrong signal matched")
	}
	if p.Rules[0].Conditions[2].compiled == nil {
		t.Fatal("regex was not precompiled")
	}
	if rule.Conditions[2].compiled != nil {
		t.Fatal("Compile mutated caller policy")
	}
}

func TestPhase1InvalidConditions(t *testing.T) {
	for _, c := range []Condition{
		{Field: FieldMethod, Operator: OpEquals, Value: "get"},
		{Field: FieldPath, Operator: OpEquals, Value: "/a/../b"},
		{Field: FieldRequestClass, Operator: OpEquals, Value: "billing"},
		{Field: FieldSignal, Operator: OpEquals, Value: "made_up"},
		{Field: FieldAllowlisted, Operator: OpEquals, Value: "false"},
	} {
		if ValidateRule(Rule{Action: ActionBlock, Conditions: []Condition{c}}, 100) == nil {
			t.Errorf("accepted invalid %+v", c)
		}
	}
}
