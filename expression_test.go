package delayed_job

import (
	"reflect"
	"testing"
)

var all_get_tests = []struct {
	expression string
	input      interface{}
	e          string
	value      interface{}
}{{expression: ``, input: "123", value: "123"},
	{expression: `a`, input: []interface{}{"a", 12}, value: 12, e: notMap.Error()},
	{expression: `[1]`, input: []interface{}{"a", 12}, value: 12},
	{expression: `[-1]`, input: []interface{}{"a", 12}, value: 12, e: invalidIndex.Error()},
	{expression: `[0]`, input: []interface{}{"a", 12}, value: "a"},
	{expression: `[0]`, input: []interface{}{map[string]interface{}{"a": 12}}, value: map[string]interface{}{"a": 12}},
	{expression: `[0].a`, input: []interface{}{map[string]interface{}{"a": 12}}, value: 12},
	{expression: `[0.a`, input: []interface{}{map[string]interface{}{"a": 12}}, e: "sytex error: '[0.a'"},
	{expression: `[0].`, input: []interface{}{map[string]interface{}{"a": 12}}, e: "sytex error: '[0].'"},
	{expression: `[0].b`, input: []interface{}{map[string]interface{}{"a": 12}}, e: notFound.Error()},
	{expression: `b`, input: map[string]interface{}{"a": 12}, e: notFound.Error()},
	{expression: `a`, input: map[string]interface{}{"a": 12}, value: 12}}

func TestToSimpleValue(t *testing.T) {
	for i, test := range all_get_tests {
		ret, e := newValueHolder(test.input).simpleValue(test.expression)
		if nil != e {
			if 0 == len(test.e) {
				t.Errorf("test all_get_tests[%v] failed, %v", i, e)
			} else if test.e != e.Error() {
				t.Errorf("test all_get_tests[%v] failed, excepted error is '%v', actual error is '%v'", i, test.e, e)
			}
			continue
		}

		if !reflect.DeepEqual(ret, test.value) {
			t.Errorf("test all_get_tests[%v] failed, excepted value is '%v', actual is '%v'", i, test.value, ret)
		}
	}
}
