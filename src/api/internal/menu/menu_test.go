package menu_test

import (
	"strings"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

// SampleTree is a small menu used across tests.
func sampleTree() []map[string]any {
	opt := func(th, en string, delta int) map[string]any {
		return map[string]any{"name_th": th, "name_en": en, "price_delta_satang": delta}
	}
	return []map[string]any{
		{"name_th": "ก๋วยเตี๋ยว", "name_en": "Noodles", "items": []map[string]any{
			{"name_th": "ผัดไทย", "name_en": "Pad Thai", "price_satang": 12000, "option_groups": []map[string]any{
				{"name_th": "เนื้อสัตว์", "name_en": "Protein", "min_choices": 1, "max_choices": 1,
					"options": []map[string]any{opt("ไก่", "Chicken", 0), opt("กุ้ง", "Shrimp", 3000)}},
				{"name_th": "เพิ่ม", "name_en": "Extras", "min_choices": 0, "max_choices": 2,
					"options": []map[string]any{opt("ไข่", "Egg", 1000), opt("ถั่ว", "Peanuts", 500)}},
			}},
			{"name_th": "ต้มยำ", "name_en": "Tom Yum", "price_satang": 15000, "option_groups": []map[string]any{}},
		}},
		{"name_th": "เครื่องดื่ม", "name_en": "Drinks", "items": []map[string]any{
			{"name_th": "ชาไทย", "name_en": "Thai Tea", "price_satang": 6000, "option_groups": []map[string]any{}},
		}},
	}
}

func put(e *testenv.Env, cookie string, rev int, cats any) testenv.Resp {
	return e.Do("PUT", "/api/v1/branches/"+e.Branch+"/menu", testenv.StaffCookie(cookie), testenv.AdminOrigin, "",
		map[string]any{"expected_revision": rev, "categories": cats})
}

// toInput converts a menu response into a PUT body (ids retained).
func toInput(r testenv.Resp) []map[string]any {
	var out []map[string]any
	for _, c := range r.Get("categories").([]any) {
		cm := c.(map[string]any)
		var items []map[string]any
		for _, it := range cm["items"].([]any) {
			im := it.(map[string]any)
			var groups []map[string]any
			for _, g := range im["option_groups"].([]any) {
				gm := g.(map[string]any)
				var opts []map[string]any
				for _, o := range gm["options"].([]any) {
					om := o.(map[string]any)
					opts = append(opts, map[string]any{"id": om["id"], "name_th": om["name_th"], "name_en": om["name_en"], "price_delta_satang": om["price_delta_satang"]})
				}
				groups = append(groups, map[string]any{"id": gm["id"], "name_th": gm["name_th"], "name_en": gm["name_en"],
					"min_choices": gm["min_choices"], "max_choices": gm["max_choices"], "options": opts})
			}
			if groups == nil {
				groups = []map[string]any{}
			}
			items = append(items, map[string]any{"id": im["id"], "name_th": im["name_th"], "name_en": im["name_en"], "price_satang": im["price_satang"], "option_groups": groups})
		}
		out = append(out, map[string]any{"id": cm["id"], "name_th": cm["name_th"], "name_en": cm["name_en"], "items": items})
	}
	return out
}

func item(r testenv.Resp, en string) map[string]any {
	for _, c := range r.Get("categories").([]any) {
		for _, it := range c.(map[string]any)["items"].([]any) {
			if it.(map[string]any)["name_en"] == en {
				return it.(map[string]any)
			}
		}
	}
	return nil
}

func changedRev(e *testenv.Env, id string) int {
	return testenv.Scalar[int](e, "SELECT changed_revision FROM menu_items WHERE id = $1", id)
}

// TestMenuReplaceValidation (MEN-001): invalid configurations and roles.
func TestMenuReplaceValidation(t *testing.T) {
	e := testenv.New(t)
	bad := sampleTree()
	g := bad[0]["items"].([]map[string]any)[0]["option_groups"].([]map[string]any)[0]
	g["min_choices"], g["max_choices"] = 2, 1
	if r := put(e, e.Manager, 1, bad); r.Status != 422 {
		t.Fatalf("min > max accepted: %d %s", r.Status, r.Raw)
	}
	bad = sampleTree()
	bad[0]["items"].([]map[string]any)[0]["option_groups"].([]map[string]any)[0]["max_choices"] = 3 // only 2 options
	if r := put(e, e.Manager, 1, bad); r.Status != 422 {
		t.Fatalf("max > options accepted: %d", r.Status)
	}
	bad = sampleTree()
	bad[1]["items"].([]map[string]any)[0]["price_satang"] = -1
	if r := put(e, e.Manager, 1, bad); r.Status != 422 {
		t.Fatalf("negative price accepted: %d", r.Status)
	}
	bad = sampleTree()
	bad[1]["name_en"] = strings.Repeat("x", 81)
	if r := put(e, e.Manager, 1, bad); r.Status != 422 {
		t.Fatalf("long name accepted: %d", r.Status)
	}
	for _, c := range []string{e.Host, e.Kitchen} {
		if r := put(e, c, 1, sampleTree()); r.Status != 403 {
			t.Fatalf("non-manager replaced the menu: %d", r.Status)
		}
	}
	if r := put(e, e.Manager, 7, sampleTree()); r.Code() != "VERSION_CONFLICT" {
		t.Fatalf("stale revision: %d %s", r.Status, r.Raw)
	}
	bad = sampleTree()
	bad[0]["id"] = "0198f0c0-0000-7000-8000-00000000dead"
	if r := put(e, e.Manager, 1, bad); r.Status != 422 {
		t.Fatalf("unknown id accepted: %d %s", r.Status, r.Raw)
	}
}

// TestMenuReplaceCreatesUpdatesRetires: ids are kept, omitted entities
// retire (not delete) and disappear from the public menu.
func TestMenuReplaceCreatesUpdatesRetires(t *testing.T) {
	e := testenv.New(t)
	r := put(e, e.Manager, 1, sampleTree())
	if r.Status != 200 || r.Num("revision") != 2 || r.Len("categories") != 2 {
		t.Fatalf("create: %d %s", r.Status, r.Raw)
	}
	in := toInput(r)
	in[0]["items"] = in[0]["items"].([]map[string]any)[:1] // drop Tom Yum
	in[0]["name_en"] = "Noodle dishes"
	r2 := put(e, e.Manager, 2, in)
	if r2.Status != 200 || r2.Str("categories", 0, "name_en") != "Noodle dishes" || item(r2, "Tom Yum") != nil {
		t.Fatalf("update: %d %s", r2.Status, r2.Raw)
	}
	if item(r2, "Pad Thai")["id"] != item(r, "Pad Thai")["id"] {
		t.Fatal("item id changed across replace")
	}
	if n := e.Count("SELECT count(*) FROM menu_items WHERE name_en = 'Tom Yum' AND retired_at IS NOT NULL"); n != 1 {
		t.Fatal("omitted item was not retired")
	}
	pub := e.Do("GET", "/api/v1/branches/"+e.Branch+"/menu", "", "", "", nil)
	if pub.Status != 200 || pub.Str("currency") != "THB" || item(pub, "Tom Yum") != nil || pub.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("public menu: %d %s", pub.Status, pub.Raw)
	}
}

// TestChangedRevisionTracksChargeChanges: renames do not invalidate carts;
// price/option changes do, for that item only.
func TestChangedRevisionTracksChargeChanges(t *testing.T) {
	e := testenv.New(t)
	r := put(e, e.Manager, 1, sampleTree())
	padID, teaID := item(r, "Pad Thai")["id"].(string), item(r, "Thai Tea")["id"].(string)
	if changedRev(e, padID) != 2 {
		t.Fatalf("new item changed_revision = %d", changedRev(e, padID))
	}
	in := toInput(r)
	in[0]["items"].([]map[string]any)[0]["name_en"] = "Pad Thai (classic)"
	r = put(e, e.Manager, 2, in)
	if changedRev(e, padID) != 2 {
		t.Fatal("rename bumped changed_revision")
	}
	in = toInput(r)
	in[0]["items"].([]map[string]any)[0]["option_groups"].([]map[string]any)[0]["options"].([]map[string]any)[1]["price_delta_satang"] = 3500
	put(e, e.Manager, 3, in)
	if changedRev(e, padID) != 4 || changedRev(e, teaID) != 2 {
		t.Fatalf("option price change: pad=%d tea=%d", changedRev(e, padID), changedRev(e, teaID))
	}
}

// TestAvailabilityToggle: manager and kitchen may toggle; hosts may not;
// the item version guards concurrent toggles.
func TestAvailabilityToggle(t *testing.T) {
	e := testenv.New(t)
	r := put(e, e.Manager, 1, sampleTree())
	tea := item(r, "Thai Tea")
	id := tea["id"].(string)
	path := "/api/v1/menu-items/" + id + "/availability"
	v := int(tea["version"].(float64))
	if x := e.Do("PATCH", path, testenv.StaffCookie(e.Host), testenv.AdminOrigin, "", map[string]any{"expected_version": v, "sold_out": true}); x.Status != 403 {
		t.Fatalf("host toggled: %d", x.Status)
	}
	x := e.Do("PATCH", path, testenv.StaffCookie(e.Kitchen), testenv.AdminOrigin, "", map[string]any{"expected_version": v, "sold_out": true})
	if x.Status != 200 || x.Get("sold_out") != true {
		t.Fatalf("kitchen toggle: %d %s", x.Status, x.Raw)
	}
	if y := e.Do("PATCH", path, testenv.StaffCookie(e.Manager), testenv.AdminOrigin, "", map[string]any{"expected_version": v, "sold_out": false}); y.Code() != "VERSION_CONFLICT" {
		t.Fatalf("stale toggle: %d", y.Status)
	}
	pub := e.Do("GET", "/api/v1/branches/"+e.Branch+"/menu", "", "", "", nil)
	if item(pub, "Thai Tea")["sold_out"] != true || pub.Num("revision") != 3 {
		t.Fatalf("public after toggle: %s", pub.Raw)
	}
}

// TestMenuBrowseBatched: the public read uses a fixed number of statements
// regardless of menu size (no N+1), counted by a pgx query tracer.
func TestMenuBrowseBatched(t *testing.T) {
	e := testenv.New(t)
	var cats []map[string]any
	for c := range 5 {
		var items []map[string]any
		for i := range 20 {
			items = append(items, map[string]any{"name_th": "อาหาร", "name_en": "Dish " + string(rune('A'+c)) + string(rune('a'+i)), "price_satang": 5000 + i,
				"option_groups": []map[string]any{{"name_th": "ขนาด", "name_en": "Size", "min_choices": 1, "max_choices": 1,
					"options": []map[string]any{{"name_th": "เล็ก", "name_en": "S", "price_delta_satang": 0}, {"name_th": "ใหญ่", "name_en": "L", "price_delta_satang": 2000}}}}})
		}
		cats = append(cats, map[string]any{"name_th": "หมวด", "name_en": "Cat " + string(rune('A'+c)), "items": items})
	}
	if r := put(e, e.Manager, 1, cats); r.Status != 200 {
		t.Fatalf("seed: %d %s", r.Status, r.Raw)
	}
	var pub testenv.Resp
	n := e.Statements(func() { pub = e.Do("GET", "/api/v1/branches/"+e.Branch+"/menu", "", "", "", nil) })
	if pub.Status != 200 || pub.Len("categories") != 5 || pub.Len("categories", 0, "items") != 20 || pub.Len("categories", 0, "items", 0, "option_groups", 0, "options") != 2 {
		t.Fatalf("menu shape: %d", pub.Status)
	}
	// 1 branch check + 4 reads, independent of 100 items / 200 options.
	if n != 5 {
		t.Fatalf("menu read used %d statements, want 5", n)
	}
	if r := e.Do("GET", "/api/v1/branches/0198f0c0-0000-7000-8000-000000000000/menu", "", "", "", nil); r.Status != 404 {
		t.Fatalf("unknown branch: %d", r.Status)
	}
}
