package cli

import "testing"

func TestRenderRowTransformTemplateValue(t *testing.T) {
	got, err := RenderRowTransformTemplate("{{upper .Value}}", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("expected HELLO, got %q", got)
	}
}

func TestRenderRowTransformTemplateGenerator(t *testing.T) {
	got, err := RenderRowTransformTemplate("{{uuid}}", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 36 {
		t.Fatalf("expected a 36-char uuid, got %q", got)
	}
}

func TestRenderRowTransformTemplateSyntaxError(t *testing.T) {
	_, err := RenderRowTransformTemplate("{{.Value", "x")
	if err == nil {
		t.Fatal("expected a parse error for malformed template")
	}
}
