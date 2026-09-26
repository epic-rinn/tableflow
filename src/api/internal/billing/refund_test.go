package billing_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

func (f *fixture) refund(cookie, key, settlement string) testenv.Resp {
	if key == "" {
		key = testenv.NewKey()
	}
	return f.e.Do("POST", "/api/v1/settlements/"+settlement+"/refund", testenv.StaffCookie(cookie), testenv.AdminOrigin, key,
		map[string]any{"reason": "dish was cold", "external_reference": "BANK-REF-77"})
}

func (f *fixture) paid() string {
	f.e.T.Helper()
	id, v := f.order(f.tea, 1)
	f.serve(id, v)
	return f.settle().Str("id")
}

// TestConcurrentRefundOneRecord (BIL-A6 non-member part). [BIL-007]
func TestConcurrentRefundOneRecord(t *testing.T) {
	f := setup(t)
	sid := f.paid()
	key := testenv.NewKey()
	var wg sync.WaitGroup
	res := make([]testenv.Resp, 3)
	for i := range res {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k := ""
			if i == 0 {
				k = key
			}
			res[i] = f.refund(f.e.Manager, k, sid)
		}()
	}
	wg.Wait()
	created := 0
	for _, r := range res {
		switch {
		case r.Status == 201:
			created++
		case r.Code() == "ALREADY_REFUNDED":
		default:
			t.Fatalf("refund outcome: %d %s", r.Status, r.Raw)
		}
	}
	if created != 1 || f.e.Count("SELECT count(*) FROM refunds") != 1 {
		t.Fatalf("created = %d", created)
	}
	if res[0].Status == 201 {
		if r := f.refund(f.e.Manager, key, sid); r.Header.Get("Idempotency-Replayed") != "true" {
			t.Fatalf("replay: %d %s", r.Status, r.Raw)
		}
	}
	if n := f.e.Count("SELECT count(*) FROM audit_events WHERE action = 'settlement.refunded' AND reason = 'dish was cold'"); n != 1 {
		t.Fatalf("refund audit rows = %d", n)
	}
}

// TestRefundRoles: cashier-only staff and guests cannot refund.
func TestRefundRoles(t *testing.T) {
	f := setup(t)
	sid := f.paid()
	if r := f.refund(f.e.Cashier, "", sid); r.Status != 403 {
		t.Fatalf("cashier refund: %d", r.Status)
	}
	if r := f.e.Do("POST", "/api/v1/settlements/"+sid+"/refund", "", testenv.PWAOrigin, testenv.NewKey(),
		map[string]any{"reason": "x", "external_reference": "y"}); r.Status != 401 && r.Status != 403 {
		t.Fatalf("anonymous refund: %d", r.Status)
	}
	if r := f.e.StaffSend("POST", f.e.Manager, "/api/v1/settlements/"+sid+"/refund", map[string]any{"reason": "", "external_reference": ""}); r.Status != 422 {
		t.Fatalf("empty refund: %d", r.Status)
	}
	if f.e.Count("SELECT count(*) FROM refunds") != 0 {
		t.Fatal("refund recorded")
	}
}

// TestReceiptImmutable: refunds keep the settlement and snapshot; the paid
// visit never reopens; historical prices stay after menu changes.
func TestReceiptImmutable(t *testing.T) {
	f := setup(t)
	sid := f.paid()
	before := f.e.StaffGet(f.e.Cashier, "/api/v1/settlements/"+sid)
	if before.Status != 200 || before.Num("total_satang") != 6000 || before.Get("refund") != nil {
		t.Fatalf("receipt: %d %s", before.Status, before.Raw)
	}
	f.e.Exec("UPDATE menu_items SET price_satang = 1 WHERE id = $1", f.tea)
	f.setPolicy("exclusive", 700, 1000)
	if r := f.refund(f.e.Manager, "", sid); r.Status != 201 || r.Num("refund", "amount_satang") != 6000 {
		t.Fatalf("refund: %d %s", r.Status, r.Raw)
	}
	after := f.e.StaffGet(f.e.Cashier, "/api/v1/settlements/"+sid)
	if after.Num("total_satang") != 6000 || after.Num("amount_satang") != 6000 || after.Num("lines", 0, "unit_price_satang") != 6000 ||
		after.Str("receipt_reference") != before.Str("receipt_reference") || after.Str("refund", "reason") != "dish was cold" {
		t.Fatalf("receipt after refund: %s", after.Raw)
	}
	if s := testenv.Scalar[string](f.e, "SELECT state FROM visits WHERE id = $1", f.visit); s != "paid" {
		t.Fatalf("visit state after refund = %s", s)
	}
	bv := f.bill(f.e.Cashier).Num("bill_version")
	if r := f.e.StaffSend("POST", f.e.Cashier, f.path("/settlement/reopen"), map[string]any{"expected_version": bv, "reason": "undo"}); r.Code() != "ALREADY_PAID" {
		t.Fatalf("paid bill reopened: %d %s", r.Status, r.Raw)
	}
	if r := f.begin(f.e.Cashier, bv); r.Code() != "ALREADY_PAID" {
		t.Fatalf("paid bill re-begun: %d %s", r.Status, r.Raw)
	}
}

// TestReceiptLookup: branch-scoped list, reference filter and keyset pages.
func TestReceiptLookup(t *testing.T) {
	f := setup(t)
	refs := map[string]bool{}
	for i := range 3 {
		if i > 0 {
			f.visit, f.token = f.e.SeatWalkIn(f.e.Table(fmt.Sprintf("P%d", i), 2), 1)
		}
		id, v := f.order(f.tea, 1)
		f.serve(id, v)
		refs[f.settle().Str("receipt_reference")] = true
	}
	list := "/api/v1/branches/" + f.e.Branch + "/settlements"
	seen := map[string]bool{}
	cursor := ""
	for range 5 {
		path := list + "?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := f.e.StaffGet(f.e.Cashier, path)
		if r.Status != 200 {
			t.Fatalf("list: %d %s", r.Status, r.Raw)
		}
		for i := range r.Len("items") {
			seen[r.Str("items", i, "receipt_reference")] = true
		}
		if cursor = r.Str("next_cursor"); cursor == "" {
			break
		}
	}
	if len(seen) != 3 {
		t.Fatalf("listed %d receipts", len(seen))
	}
	for ref := range refs {
		r := f.e.StaffGet(f.e.Manager, list+"?receipt_reference="+ref)
		if r.Len("items") != 1 || r.Str("items", 0, "receipt_reference") != ref {
			t.Fatalf("filter: %s", r.Raw)
		}
		break
	}
	if r := f.e.StaffGet(f.e.Cashier, list+"?receipt_reference=bogus"); r.Status != 422 {
		t.Fatalf("bad reference: %d", r.Status)
	}
	if r := f.e.StaffGet(f.e.Kitchen, list); r.Status != 403 {
		t.Fatalf("kitchen list: %d", r.Status)
	}
	if r := f.e.StaffGet(f.e.Cashier, "/api/v1/branches/00000000-0000-4000-8000-000000000000/settlements"); r.Status != 404 {
		t.Fatalf("other branch: %d", r.Status)
	}
}
