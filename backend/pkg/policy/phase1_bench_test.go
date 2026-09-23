package policy

import "testing"

func BenchmarkEvaluateCompiledWorstCase(b *testing.B) {
	rules := make([]Rule, 200)
	for i := range rules {
		rules[i] = Rule{ID: "unmatched", Enabled: true, Action: ActionBlock, Conditions: []Condition{{Field: FieldPath, Operator: OpMatches, Value: `^/admin/[0-9]+$`}}}
	}
	p, err := Compile(&Policy{Rules: rules})
	if err != nil {
		b.Fatal(err)
	}
	f := Facts{Path: "/shop/product", Method: "GET", Class: ClassBrowse}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Evaluate(p, f)
	}
}
