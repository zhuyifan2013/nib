package binding

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "把生成结果写入 testdata/golden 作为基准")

// updateGolden 在 -update 模式下把 out 目录内容递归同步进 golden 目录。
func updateGolden(t *testing.T, out, golden string) {
	t.Helper()
	os.RemoveAll(golden)
	err := filepath.Walk(out, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(out, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(golden, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func loadExample(t *testing.T) *Package {
	t.Helper()
	pkg, err := Load("testdata/example")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return pkg
}

func TestLoadExample(t *testing.T) {
	pkg := loadExample(t)

	if got, want := len(pkg.Services), 3; got != want {
		t.Fatalf("服务数 = %d, 期望 %d", got, want)
	}
	byName := map[string]Service{}
	for _, s := range pkg.Services {
		byName[s.Name] = s
	}

	greet := byName["GreetService"]
	if greet.Name == "" {
		t.Fatal("缺少 GreetService")
	}
	var m map[string]Method
	collect := func(s Service) {
		m = map[string]Method{}
		for _, mm := range s.Methods {
			m[mm.Name] = mm
		}
	}
	collect(greet)

	cases := []struct {
		method string
		kind   Kind
		params []Param
		result string
	}{
		{"Greet", KindCall, []Param{{"args", "GreetArgs"}}, "GreetResult"},
		{"Sum", KindCall, []Param{{"a", "number"}, {"b", "number"}}, "number"},
		{"Check", KindCall, []Param{{"flag", "boolean"}, {"raw", "unknown"}}, "boolean"},
		{"List", KindCall, []Param{{"ids", "number[]"}}, "Status[]"},
		{"Ping", KindCall, nil, "string"},
		{"StatusOf", KindCall, []Param{{"base", "Base"}}, "{ code: number; id: number }"},
	}
	for _, c := range cases {
		got, ok := m[c.method]
		if !ok {
			t.Errorf("缺少方法 %s", c.method)
			continue
		}
		if got.Kind != c.kind {
			t.Errorf("%s.Kind = %q, 期望 %q", c.method, got.Kind, c.kind)
		}
		if strings.Join(paramsSig(got.Params), "|") != strings.Join(paramsSig(c.params), "|") {
			t.Errorf("%s.Params = %v, 期望 %v", c.method, got.Params, c.params)
		}
		if got.Result != c.result {
			t.Errorf("%s.Result = %q, 期望 %q", c.method, got.Result, c.result)
		}
	}
	if _, ok := m["hidden"]; ok {
		t.Error("hidden 不应参与生成")
	}
	if m["Greet"].Doc != "Greet 问候。" {
		t.Errorf("Greet.Doc = %q", m["Greet"].Doc)
	}

	collect(byName["TaskService"])
	if m["RunPipeline"].Kind != KindStream || m["RunPipeline"].Elem != "number" {
		t.Errorf("RunPipeline = %+v", m["RunPipeline"])
	}
	collect(byName["BusService"])
	if m["FileChanged"].Kind != KindEvent || m["FileChanged"].Elem != "GreetArgs" {
		t.Errorf("FileChanged = %+v", m["FileChanged"])
	}

	// 命名结构体：GreetArgs / GreetResult / Base 去重出现。
	names := map[string]bool{}
	for _, s := range pkg.Structs {
		names[s.Name] = true
	}
	for _, want := range []string{"GreetArgs", "GreetResult", "Base"} {
		if !names[want] {
			t.Errorf("Structs 缺少 %s（实际 %v）", want, names)
		}
	}
}

func paramsSig(ps []Param) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Name+":"+p.Type)
	}
	return out
}

func TestGenerateGolden(t *testing.T) {
	pkg := loadExample(t)
	out := t.TempDir()
	if _, err := Generate(pkg, out); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	golden := "testdata/golden"
	if *update {
		updateGolden(t, out, golden)
	}
	entries, err := os.ReadDir(golden)
	if err != nil {
		t.Fatalf("读取 golden: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("testdata/golden 为空：先运行 go test ./binding -update 生成基准")
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // services/ 子目录走下面的递归对比
		}
		assertSameFile(t, out, golden, e.Name())
	}
	svc, err := os.ReadDir(filepath.Join(golden, "services"))
	if err == nil {
		for _, e := range svc {
			assertSameFile(t, out, golden, filepath.Join("services", e.Name()))
		}
	}
}

func assertSameFile(t *testing.T, out, golden, rel string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join(golden, rel))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(out, rel))
	if err != nil {
		t.Errorf("输出缺少 %s", rel)
		return
	}
	if string(got) != string(want) {
		t.Errorf("%s 与基准不符（跑 -update 后人工核对 diff）", rel)
	}
}

func TestGenerateIncremental(t *testing.T) {
	pkg := loadExample(t)
	out := t.TempDir()
	rep, err := Generate(pkg, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Written) == 0 {
		t.Fatal("首次生成应全部 write")
	}
	// 第二次生成：内容不变，不应重写任何文件。
	rep2, err := Generate(pkg, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep2.Written) != 0 || len(rep2.Unchanged) != len(rep.Written)+len(rep.Unchanged) {
		t.Errorf("增量生成重写了文件：written=%v", rep2.Written)
	}

	// 删除一个服务后重新生成：残留文件应被清理。
	var shrunk Package
	for _, s := range pkg.Services {
		if s.Name != "BusService" {
			shrunk.Services = append(shrunk.Services, s)
		}
	}
	shrunk.Structs = pkg.Structs
	rep3, err := Generate(&shrunk, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep3.Removed) != 1 || rep3.Removed[0] != filepath.Join("services", "BusService.ts") {
		t.Errorf("Removed = %v", rep3.Removed)
	}
}

func TestLoadRejectsBadSignatures(t *testing.T) {
	dir := t.TempDir()
	src := `package bad

type Svc struct{}

func (s *Svc) Variadic(prefix string, rest ...int) (string, error) { return "", nil }
func (s *Svc) Triple() (int, int, error) { return 0, 0, nil }
func (s *Svc) BadKind() (string, error) { return "", nil }
`
	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("应拒绝非法签名")
	}
	for _, want := range []string{"Variadic", "Triple"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息应提到 %s：%v", want, err)
		}
	}
}
