package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

func newBaselineHandler(t *testing.T) (*BaselineHandler, store.Interface) {
	t.Helper()
	st := store.NewMemory()
	return NewBaselineHandler(st, "mac", eventbus.New()), st
}

func seedReportHost(t *testing.T, st store.Interface) string {
	t.Helper()
	// 注意：memory store 会自行分配主机 ID（mem-host-N），以返回值为准
	host := &models.Host{Name: "node-1", MAC: "00:11:22:33:44:55"}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	return host.ID
}

func postReport(t *testing.T, h *BaselineHandler, query, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/baselines/report?"+query, strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Report(w, req)
	return w
}

func TestBaselineReportCreate(t *testing.T) {
	h, st := newBaselineHandler(t)
	hostID := seedReportHost(t, st)

	w := postReport(t, h, "mac=00:11:22:33:44:55",
		`{"script_name":"init-ntp","seq":1,"exit_code":0,"duration_ms":120,"output_tail":"ok"}`)
	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Result().StatusCode, w.Body.String())
	}

	reports, err := st.ListBaselineReports(t.Context(), hostID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	r := reports[0]
	if r.ScriptName != "init-ntp" || r.Seq != 1 || r.ExitCode != 0 || r.DurationMs != 120 || r.OutputTail != "ok" {
		t.Fatalf("unexpected report: %+v", r)
	}
}

func TestBaselineReportUnknownHostSilent204(t *testing.T) {
	h, st := newBaselineHandler(t)

	// 主机不存在 → 204 静默丢弃，不阻断装机
	w := postReport(t, h, "mac=aa:bb:cc:dd:ee:ff", `{"script_name":"s","seq":1,"exit_code":0}`)
	if w.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Result().StatusCode)
	}

	reports, err := st.ListBaselineReports(t.Context(), "host-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected 0 reports, got %d", len(reports))
	}
}

func TestBaselineReportOutputTailTruncated(t *testing.T) {
	h, st := newBaselineHandler(t)
	hostID := seedReportHost(t, st)

	big := strings.Repeat("x", 6000)
	w := postReport(t, h, "mac=00:11:22:33:44:55",
		fmt.Sprintf(`{"script_name":"s","seq":1,"exit_code":0,"output_tail":"%s"}`, big))
	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Result().StatusCode)
	}

	reports, _ := st.ListBaselineReports(t.Context(), hostID)
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if len(reports[0].OutputTail) != baselineOutputTailLimit {
		t.Fatalf("expected tail truncated to %d bytes, got %d", baselineOutputTailLimit, len(reports[0].OutputTail))
	}
}

func TestBaselineReportUTF8Boundary(t *testing.T) {
	// 截断点落在多字节字符中间时不能产生非法 UTF-8
	s := strings.Repeat("a", baselineOutputTailLimit-1) + "汉"
	got := truncateTail(s, baselineOutputTailLimit)
	if strings.ContainsRune(got, '�') {
		t.Fatalf("invalid utf-8 after truncate: %q", got[:8])
	}
	if len(got) > baselineOutputTailLimit {
		t.Fatalf("expected <= %d bytes, got %d", baselineOutputTailLimit, len(got))
	}
}

func TestBaselineReportPruneKeepsLatest100(t *testing.T) {
	h, st := newBaselineHandler(t)
	hostID := seedReportHost(t, st)

	// 写入 105 条，落库后应滚动清理只留最近 100 条
	for i := 1; i <= 105; i++ {
		w := postReport(t, h, "mac=00:11:22:33:44:55",
			fmt.Sprintf(`{"script_name":"s%d","seq":%d,"exit_code":0}`, i, i))
		if w.Result().StatusCode != http.StatusCreated {
			t.Fatalf("report %d: expected 201, got %d", i, w.Result().StatusCode)
		}
	}

	reports, _ := st.ListBaselineReports(t.Context(), hostID)
	if len(reports) != baselineReportKeep {
		t.Fatalf("expected %d reports after prune, got %d", baselineReportKeep, len(reports))
	}
	// 倒序：最新的一条（s105）在最前，最旧保留 s6
	if reports[0].ScriptName != "s105" {
		t.Fatalf("expected newest report first, got %s", reports[0].ScriptName)
	}
	if reports[len(reports)-1].ScriptName != "s6" {
		t.Fatalf("expected oldest kept report s6, got %s", reports[len(reports)-1].ScriptName)
	}
}

func TestBaselineReportSNIdentity(t *testing.T) {
	st := store.NewMemory()
	h := NewBaselineHandler(st, "sn", eventbus.New())
	host := &models.Host{Name: "node-sn", MAC: "00:11:22:33:44:66", SN: "SN-001"}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}

	w := postReport(t, h, "sn=SN-001", `{"script_name":"s","seq":1,"exit_code":1,"output_tail":"boom"}`)
	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Result().StatusCode)
	}
	reports, _ := st.ListBaselineReports(t.Context(), host.ID)
	if len(reports) != 1 || reports[0].ExitCode != 1 || reports[0].OutputTail != "boom" {
		t.Fatalf("unexpected reports: %+v", reports)
	}
}

func TestListBaselineReportsEndpoint(t *testing.T) {
	h, st := newBaselineHandler(t)
	hostID := seedReportHost(t, st)
	postReport(t, h, "mac=00:11:22:33:44:55", `{"script_name":"a","seq":1,"exit_code":0}`)
	postReport(t, h, "mac=00:11:22:33:44:55", `{"script_name":"b","seq":2,"exit_code":1,"output_tail":"boom"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/"+hostID+"/baseline-reports", nil)
	req = withChiParams(req, "id", hostID)
	w := httptest.NewRecorder()
	h.ListReports(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	out := w.Body.String()
	// 倒序：b 在前 a 在后
	if !strings.Contains(out, `"script_name":"b"`) || !strings.Contains(out, `"script_name":"a"`) {
		t.Fatalf("missing reports in response: %s", out)
	}
	if strings.Index(out, `"script_name":"b"`) > strings.Index(out, `"script_name":"a"`) {
		t.Fatalf("expected desc order (b before a): %s", out)
	}
}

// 校验 pull.sh 聚合产物内嵌了 report 调用（地址用请求方 Host 构造，身份参数透传）。
func TestPullShellScriptContainsReport(t *testing.T) {
	st := store.NewMemory()
	h := NewBaselineHandler(st, "mac", eventbus.New())
	host := &models.Host{Name: "node-1", MAC: "00:11:22:33:44:55"}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	sc := &models.Script{Name: "init-ntp", Type: "shell", Content: "echo ok"}
	if err := st.CreateScript(t.Context(), sc); err != nil {
		t.Fatal(err)
	}
	bl := &models.Baseline{ID: "bl-1", Name: "bl-1"}
	if err := st.CreateBaseline(t.Context(), bl); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBaselineScripts(t.Context(), "bl-1", []models.BaselineScriptAssignment{
		{BaselineID: "bl-1", ScriptID: sc.ID, Seq: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := host.SetBaselineIDs([]string{"bl-1"}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://10.0.0.10:8080/api/v1/baselines/pull.sh?mac=00:11:22:33:44:55", nil)
	w := httptest.NewRecorder()
	h.PullShellScript(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	out := w.Body.String()
	if !strings.Contains(out, "_pxelab_report_url='http://10.0.0.10:8080/api/v1/baselines/report?mac=00:11:22:33:44:55'") {
		t.Fatalf("report URL missing or wrong:\n%s", out)
	}
	if !strings.Contains(out, `curl -fsS -m 5 -H 'Content-Type: application/json'`) {
		t.Fatalf("curl report call missing:\n%s", out)
	}
	if !strings.Contains(out, "wget -q -T 5") {
		t.Fatalf("wget fallback missing:\n%s", out)
	}
	// 上报调用带脚本名/序号/退出码/耗时/日志文件
	if !strings.Contains(out, `_pxelab_report 'init-ntp' 1 "$_pxelab_ec" "$_pxelab_dur" /tmp/pxelab-init-0.log`) {
		t.Fatalf("per-script report invocation missing:\n%s", out)
	}
	// 单条失败不中断语义保留（上报后继续执行，不 exit）
	if strings.Contains(out, "exit 1") {
		t.Fatalf("unexpected hard exit:\n%s", out)
	}
}

// 校验 pull.ps1 聚合产物内嵌了 report 调用。
func TestPullPowerShellScriptContainsReport(t *testing.T) {
	st := store.NewMemory()
	h := NewBaselineHandler(st, "mac", eventbus.New())
	host := &models.Host{Name: "node-1", MAC: "00:11:22:33:44:55"}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	sc := &models.Script{Name: "win-init", Type: "powershell", Content: "Write-Output ok"}
	if err := st.CreateScript(t.Context(), sc); err != nil {
		t.Fatal(err)
	}
	if err := host.SetScriptIDs([]uint{sc.ID}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://10.0.0.10:8080/api/v1/baselines/pull.ps1?mac=00:11:22:33:44:55", nil)
	w := httptest.NewRecorder()
	h.PullPowerShellScript(w, req)
	out := w.Body.String()
	if !strings.Contains(out, "$PxeLabReportUrl = 'http://10.0.0.10:8080/api/v1/baselines/report?mac=00:11:22:33:44:55'") {
		t.Fatalf("report URL missing:\n%s", out)
	}
	if !strings.Contains(out, "Invoke-RestMethod") || !strings.Contains(out, "Send-PxeLabReport 'win-init' 1 $_pxelabEc $_pxelabDur $_pxelabOut") {
		t.Fatalf("ps report invocation missing:\n%s", out)
	}
}
