package config

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestElementStylesCascade(t *testing.T) {
	rt := mustLoadThemeTestConfig(t, `
gopdf.theme = {
  radius = 4,
  elements = {
    row = { text = "#102030", fill = "#eeeeee" },
    row_selected = { secondary = "accent" },
    panel = { radius = { 10, 10, 2, 2 } },
  },
}
`)
	theme := rt.Config().Theme
	selected := theme.Style(ElementRowSelected)
	// A row's override carries to the selected row, but the selected row's
	// own defaults outrank it, as a more specific rule would in CSS.
	if got := selected.Text.V; got != (Color{RGB: [3]uint8{0x10, 0x20, 0x30}, Alpha: 1}) {
		t.Errorf("row_selected text = %+v, want the row's", got)
	}
	if got := selected.Fill.V; got != paletteColor("accent", 0.16) {
		t.Errorf("row_selected fill = %+v, want its own default", got)
	}
	if got := selected.Radius.V; got != corners(3) {
		t.Errorf("row_selected radius = %v, want three quarters of the theme's", got)
	}
	if got := selected.Secondary.V; got != paletteColor("accent", 1) {
		t.Errorf("row_selected secondary = %+v", got)
	}
	// The header follows its panel's top corners.
	if got := theme.Style(ElementHeader).Radius.V; got != (Corners{10, 10, 0, 0}) {
		t.Errorf("header radius = %v, want the panel's top corners", got)
	}
	// Completion starts from the panel.
	if got := theme.Style(ElementCompletion).Radius.V; got != (Corners{10, 10, 2, 2}) {
		t.Errorf("completion radius = %v, want the panel's", got)
	}
}

func TestEveryStyleSetsEveryProperty(t *testing.T) {
	for _, name := range ThemeNames() {
		theme := mustPreset(t, name)
		for e := range ElementCount {
			style := theme.Style(e)
			for _, p := range styleProps {
				if !p.isSet(&style) {
					t.Errorf("%s: %s.%s is unset", name, e, p.name)
				}
			}
		}
	}
}

func TestStatusBarStyleIsAPreset(t *testing.T) {
	theme := mustPreset(t, DefaultTheme)
	if status := theme.Style(ElementStatus); status.Floating.V || status.Fill.V.Name != "status_bar" || status.BorderSides.V != SideTop {
		t.Fatalf("bar status = %+v", status)
	}
	theme.StatusBarStyle = "pill"
	if status := theme.Style(ElementStatus); !status.Floating.V || status.Fill.V.Alpha != 0 || status.Margin.V != uniform(10) {
		t.Fatalf("pill status = %+v", status)
	}
	if left := theme.Style(ElementStatusLeft); left.Shape.V.Kind != "pill" || left.Fill.V.Name != "status_bar" {
		t.Fatalf("pill status_left = %+v", left)
	}
	if right := theme.Style(ElementStatusRight); right.Shape.V.Kind != "pill" || right.Text.V.Name != "muted" {
		t.Fatalf("status_right should start from status_left: %+v", right)
	}
	// What the theme sets goes over the preset.
	theme.Elements[ElementStatus].Floating = some(false)
	if theme.Style(ElementStatus).Floating.V {
		t.Fatal("status.floating = false did not override the pill preset")
	}
	theme.Radius = 0
	if left := theme.Style(ElementStatusLeft); left.Shape.V.Kind != "rect" {
		t.Fatalf("square theme pill shape = %+v", left.Shape.V)
	}
}

func TestElementPropertiesParse(t *testing.T) {
	rt := mustLoadThemeTestConfig(t, `
local e = gopdf.theme.elements
e.panel.fill = "accent/16"
e.panel.border = { width = 2, color = "#11223344", sides = { "top", "left" } }
e.panel.shadow = "0 3 8 shadow, 1 2 3 4 #000000"
e.panel.padding = { 1, 2 }
e.header.padding = { 1, 2, 3, 4 }
e.hint.radius = { 1, 2, 3, 4 }
e.hint.shadow = { { y = 2, blur = 4 }, { x = 1, color = "accent" } }
e.hint.fill = "none"
e.row.shape = "M0,0 H100% V100%-4 L50%,100% L0,100%-4 Z"
e.cursor.width = 2
e.status.opacity = 5
e.status_left.border = false
assert(e.panel.fill == "accent/16", e.panel.fill)
assert(e.panel.border.width == 2)
assert(e.panel.border.sides == "top left", e.panel.border.sides)
assert(e.panel.shadow[2].spread == 4)
assert(e.panel.padding[2] == 2)
assert(e.status.opacity == 1)
`)
	theme := rt.Config().Theme
	panel := theme.Style(ElementPanel)
	want := Shadow{N: 2, Layers: [maxShadowLayers]ShadowLayer{
		{Y: 3, Blur: 8, Color: Color{Name: "shadow", Alpha: 1}},
		{X: 1, Y: 2, Blur: 3, Spread: 4, Color: Color{Alpha: 1}},
	}}
	if panel.Shadow.V != want {
		t.Errorf("panel shadow = %+v", panel.Shadow.V)
	}
	if panel.BorderColor.V != (Color{RGB: [3]uint8{0x11, 0x22, 0x33}, Alpha: float64(0x44) / 255}) || panel.BorderSides.V != SideTop|SideLeft {
		t.Errorf("panel border = %+v %v", panel.BorderColor.V, panel.BorderSides.V)
	}
	if got := theme.Style(ElementHeader).Padding.V; got != (Insets{1, 2, 3, 4}) {
		t.Errorf("header padding = %+v", got)
	}
	hint := theme.Style(ElementHint)
	if hint.Radius.V != (Corners{1, 2, 3, 4}) || hint.Fill.V.Alpha != 0 || hint.Shadow.V.N != 2 || hint.Shadow.V.Layers[1].Color.Name != "accent" {
		t.Errorf("hint = %+v", hint)
	}
	if got := theme.Style(ElementRow).Shape.V; got.Kind != "path" || !strings.HasPrefix(got.Path, "M0,0") {
		t.Errorf("row shape = %+v", got)
	}
	if theme.Style(ElementStatusLeft).BorderWidth.V != 0 {
		t.Error("border = false left a border")
	}
}

func TestElementErrors(t *testing.T) {
	tests := []struct {
		name, lua, want string
	}{
		{"unknown element", `gopdf.theme.elements.pannel = { radius = 1 }`, `elements.pannel: unknown element`},
		{"property it does not take", `gopdf.theme = { elements = { prompt = { radius = 1 } } }`, `prompt has no property "radius"`},
		{"unknown colour", `gopdf.theme.elements.panel.fill = "acent"`, `unknown colour "acent"`},
		{"bad opacity", `gopdf.theme.elements.panel.fill = "accent/150"`, `expected a percentage`},
		{"negative radius", `gopdf.theme.elements.panel.radius = -1`, `must not be negative`},
		{"three radii", `gopdf.theme.elements.panel.radius = { 1, 2, 3 }`, `one radius or four`},
		{"bad side", `gopdf.theme.elements.panel.border = { sides = "middle" }`, `unknown side "middle"`},
		{"bad path", `gopdf.theme.elements.panel.shape = "M 0"`, `path ends where a coordinate was expected`},
		{"bad shadow", `gopdf.theme.elements.panel.shadow = "3"`, `expected x y`},
		{"shadow field", `gopdf.theme.elements.panel.shadow = { blurr = 3 }`, `unknown shadow field "blurr"`},
		{"elements not a table", `gopdf.theme.elements = 3`, `expected a table of elements`},
		{"read unknown element", `local _ = gopdf.theme.elements.nope`, `unknown element`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadThemeTestConfig(t, tt.lua)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestElementOptionsAtRuntime(t *testing.T) {
	rt := mustLoadThemeTestConfig(t, `gopdf.theme.radius = 6`)
	if got, err := rt.OptionValue("theme.elements.panel.radius"); err != nil || got != "6" {
		t.Fatalf("panel radius = %q, %v; want the theme's radius in effect", got, err)
	}
	if err := rt.SetOption("theme.elements.panel.radius", "1,2,3,4"); err != nil {
		t.Fatal(err)
	}
	if err := rt.SetOption("theme.elements.panel.border.color", "accent/50"); err != nil {
		t.Fatal(err)
	}
	if err := rt.SetOption("theme.elements.status.shadow", `"0 1 2 shadow"`); err != nil {
		t.Fatal(err)
	}
	if err := rt.ToggleOption("theme.elements.status.floating"); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"theme.elements.panel.radius":       "{ 1, 2, 3, 4 }",
		"theme.elements.panel.border.color": `"accent/50"`,
		"theme.elements.status.shadow":      `"0 1 2 0 shadow"`,
		"theme.elements.status.floating":    "true",
	} {
		if got, _ := rt.OptionValue(name); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	if err := rt.SetOption("theme.elements.prompt.radius", "3"); err == nil {
		t.Error("expected prompt.radius to be refused")
	}
	// Element properties stay out of the option list.
	for _, name := range rt.OptionNames() {
		if strings.HasPrefix(name, elementsOptionPrefix) {
			t.Fatalf("option list includes %s", name)
		}
	}
	// Assigning nil clears an override, so the theme's value shows again.
	if _, err := rt.Eval(`gopdf.theme.elements.panel.radius = nil`); err != nil {
		t.Fatal(err)
	}
	if got, _ := rt.OptionValue("theme.elements.panel.radius"); got != "6" {
		t.Fatalf("cleared panel radius = %s", got)
	}
}

func TestElementsTableRoundTrips(t *testing.T) {
	rt := mustLoadThemeTestConfig(t, `
gopdf.theme = {
  base = "classic",
  elements = {
    panel = { radius = { 1, 2, 3, 4 }, border = { color = "accent/25" }, shadow = true },
    hint = { shape = function(path, w, h) path:rect(0, 0, w, h, 3) end, opacity = 0.5 },
    status = { floating = true, margin = { 1, 2 } },
  },
}
local copy = gopdf.theme
assert(copy.elements.hint.opacity == 0.5)
gopdf.theme = gopdf.options.theme
`)
	theme := rt.Config().Theme
	cfg := Default()
	got, err := themeFromLua(&cfg, luaThemeTable(rt.state, theme))
	if err != nil {
		t.Fatal(err)
	}
	if got != theme {
		t.Fatalf("theme did not survive a trip through Lua:\n%+v\nwant %+v", got.Elements, theme.Elements)
	}
	if theme.Elements[ElementHint].Shape.V.Func == nil {
		t.Fatal("the hint's shape function was lost")
	}
	// A preset sets no element overrides, so its plain table has none.
	if tbl := luaThemeTable(rt.state, mustPreset(t, "birch")).RawGetString("elements").(*lua.LTable); tbl.Len() != 0 {
		t.Fatal("a preset's elements table should be empty")
	}
	if key, _ := luaThemeTable(rt.state, mustPreset(t, "birch")).RawGetString("elements").(*lua.LTable).Next(lua.LNil); key != lua.LNil {
		t.Fatalf("a preset's elements table holds %v", key)
	}
}

func TestParsePath(t *testing.T) {
	ops, err := ParsePath("M 0,0 h 100%-6 v10 L 50% 100%+2 q1 2 3 4 C 1,2 3,4 5,6 z")
	if err != nil {
		t.Fatal(err)
	}
	px := func(v float64) Length { return Length{Px: v} }
	want := []PathOp{
		{Op: 'M', Pts: [3]PathPoint{{px(0), px(0)}}},
		{Op: 'L', Pts: [3]PathPoint{{Length{1, -6}, px(0)}}},
		{Op: 'L', Pts: [3]PathPoint{{Length{1, -6}, px(10)}}},
		{Op: 'L', Pts: [3]PathPoint{{Length{Frac: 0.5}, Length{1, 2}}}},
		{Op: 'Q', Pts: [3]PathPoint{{Length{0.5, 1}, Length{1, 4}}, {Length{0.5, 3}, Length{1, 6}}}},
		{Op: 'C', Pts: [3]PathPoint{{px(1), px(2)}, {px(3), px(4)}, {px(5), px(6)}}},
		{Op: 'Z'},
	}
	if len(ops) != len(want) {
		t.Fatalf("ops = %+v", ops)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Errorf("op %d = %+v, want %+v", i, ops[i], want[i])
		}
	}
	// Coordinates after a move are lines.
	if ops, err := ParsePath("M0 0 10 0 10 10"); err != nil || len(ops) != 3 || ops[2].Op != 'L' {
		t.Fatalf("implicit lines = %+v, %v", ops, err)
	}
	for _, bad := range []string{"", "L 1 1", "0 0", "M 1", "M 1 1 Z 3", "M 1 1 A 1 1 0 0 0 2 2", "M 1% 1%x"} {
		if _, err := ParsePath(bad); err == nil {
			t.Errorf("ParsePath(%q) succeeded", bad)
		}
	}
}

func TestShapePathRunsTheFunction(t *testing.T) {
	rt := mustLoadThemeTestConfig(t, `
gopdf.theme.elements.hint.shape = function(path, w, h)
  path:move_to(0, 0):line_to(w, 0):quad_to(w, h, 0, h):close()
  path:ellipse(w / 2, h / 2, 2)
end
gopdf.theme.elements.row.shape = function(path, w, h) path:line_to(1, 1) end
gopdf.theme.elements.panel.shape = function(path, w, h) end
`)
	theme := rt.Config().Theme
	ops, err := rt.ShapePath(theme.Elements[ElementHint].Shape.V.Func, 40, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 4+6 || ops[1].Pts[0].X.Px != 40 || ops[2].Pts[1].Y.Px != 20 || ops[4].Op != 'M' || ops[4].Pts[0].X.Px != 22 {
		t.Fatalf("ops = %+v", ops)
	}
	if _, err := rt.ShapePath(theme.Elements[ElementRow].Shape.V.Func, 40, 20); err == nil || !strings.Contains(err.Error(), "start the path with move_to") {
		t.Fatalf("line_to first: %v", err)
	}
	if _, err := rt.ShapePath(theme.Elements[ElementPanel].Shape.V.Func, 40, 20); err == nil || !strings.Contains(err.Error(), "drew no path") {
		t.Fatalf("empty shape: %v", err)
	}
}

func TestStyleColorsFormatAsParsed(t *testing.T) {
	for _, text := range []string{"none", "accent", "accent/16", "shadow/50", "#102030", "#10203080", "muted/56.5"} {
		c, err := parseStyleColor(text)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if got := formatStyleColor(c); got != text {
			t.Errorf("%s formats as %s", text, got)
		}
	}
}

func TestElementReferencesCoverEveryElement(t *testing.T) {
	refs := ElementReferences()
	if len(refs) != int(ElementCount) {
		t.Fatalf("%d references for %d elements", len(refs), ElementCount)
	}
	for _, ref := range refs {
		if ref.Description == "" || ref.Defaults == "" {
			t.Errorf("%s is undocumented: %+v", ref.Name, ref)
		}
		for _, prop := range ref.Properties {
			if _, ok := styleProperty(prop); !ok && prop != "border" {
				t.Errorf("%s takes unknown property %s", ref.Name, prop)
			}
		}
	}
	for _, ref := range StylePropertyReferences() {
		if ref.Description == "" {
			t.Errorf("property %s is undocumented", ref.Name)
		}
	}
}
