package bench_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/thiagonache/bench"
)

func TestNonOKStatusRecordedAsFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		http.Error(rw, "ForceFailing", http.StatusTeapot)
	}))
	tester, err := bench.NewTester(
		bench.WithURL(server.URL),
		bench.WithHTTPClient(server.Client()),
		bench.WithStdout(io.Discard),
		bench.WithStderr(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = tester.Run()
	if err != nil {
		t.Fatal(err)
	}
	stats := tester.Stats()
	if stats.Requests != 1 {
		t.Errorf("want 1 request, got %d", stats.Requests)
	}
	if stats.Successes != 0 {
		t.Errorf("want 0 successes, got %d", stats.Successes)
	}
	if stats.Failures != 1 {
		t.Errorf("want 1 failure, got %d", stats.Failures)
	}
}

func TestNewTesterByDefaultIsConfiguredForDefaultNumRequests(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := bench.DefaultNumRequests
	got := tester.Requests()
	if want != got {
		t.Errorf("want tester configured for default number of requests (%d), got %d", want, got)
	}
}

func TestNewTesterConfiguresWorkChannelByDefaultAsUnbufferedChannel(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	got := len(tester.Work)
	if want != got {
		t.Errorf("want tester work channel is unbuffered, got %d", got)
	}
}

func TestNewTesterByDefaultIsConfiguredForDefaultOutputPath(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := bench.DefaultOutputPath
	got := tester.OutputPath
	if want != got {
		t.Errorf("want tester output path for default output path (%q), got %q", want, got)
	}
}

func TestNewTesterByDefaultIsConfiguredForDefaultConcurrency(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := bench.DefaultConcurrency
	got := tester.Concurrency
	if want != got {
		t.Errorf("want tester concurrency for default concurrency (%d), got %d", want, got)
	}
}

func TestNewTesterWithNConcurrentRequestsIsConfiguredForNConcurrentRequests(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithConcurrency(10),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := tester.Concurrency
	if got != 10 {
		t.Errorf("want tester configured for 10 concurrent requests, got %d", got)
	}
}

func TestFromArgsNConcurrencyConfiguresNConcurrency(t *testing.T) {
	t.Parallel()
	args := []string{"run", "-c", "10", "-u", "http://fake.url"}
	tester, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs(args),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := 10
	got := tester.Concurrency
	if want != got {
		t.Errorf("reqs: want %d, got %d", want, got)
	}
}

func TestNewTesterWithOutputPathIsConfiguredForOutputPath(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithOutputPath("/tmp"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "/tmp"
	got := tester.OutputPath
	if want != got {
		t.Errorf("want tester output path configured for /tmp, got %q", got)
	}
}

func TestNewTesterWithNRequestsIsConfiguredForNRequests(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithRequests(10),
	)
	if err != nil {
		t.Fatal(err)
	}
	got := tester.Requests()
	if got != 10 {
		t.Errorf("want tester configured for 10 requests, got %d", got)
	}
}

func TestNewTesterWithInvalidRequestsReturnsError(t *testing.T) {
	t.Parallel()
	_, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithRequests(-1),
	)
	if err == nil {
		t.Fatal("want error for invalid number of requests (-1)")
	}
}

func TestNewTesterByDefaultSetsDefaultHTTPUserAgent(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := bench.DefaultUserAgent
	got := tester.HTTPUserAgent()
	if want != got {
		t.Errorf("want default user agent (%q), got %q", want, got)
	}
}

func TestNewTesterWithUserAgentXSetsUserAgentX(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithHTTPUserAgent("CustomUserAgent"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "CustomUserAgent"
	got := tester.HTTPUserAgent()
	if want != got {
		t.Errorf("user-agent: want %q, got %q", want, got)
	}
}

func TestNewTesterByDefaultSetsDefaultHTTPClient(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}

	want := bench.DefaultHTTPClient
	got := tester.HTTPClient()
	if !cmp.Equal(want, got) {
		t.Errorf(cmp.Diff(want, got))
	}
}

func TestNewTesterWithHTTPClientXSetsHTTPClientX(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithHTTPClient(&http.Client{
			Timeout: 45 * time.Second,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := &http.Client{
		Timeout: 45 * time.Second,
	}
	got := tester.HTTPClient()
	if !cmp.Equal(want, got) {
		t.Errorf(cmp.Diff(want, got))
	}
}

func TestNewTesterWithInvalidURLReturnsError(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"bogus-no-scheme-or-domain",
		"bogus-no-host://",
		"bogus-no-scheme.fake",
	}
	for _, url := range inputs {
		_, err := bench.NewTester(
			bench.WithURL(url),
		)
		if err == nil {
			t.Errorf("want error for invalid URL %q", url)
		}
	}
}

func TestNewTesterWithValidURLReturnsNoError(t *testing.T) {
	t.Parallel()
	_, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Errorf("error not expected but found: %q", err)
	}
}

func TestRunReturnsValidStatsAndTime(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "HelloWorld")
	}))
	tester, err := bench.NewTester(
		bench.WithURL(server.URL),
		bench.WithRequests(100),
		bench.WithHTTPClient(server.Client()),
		bench.WithStdout(io.Discard),
		bench.WithStderr(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = tester.Run()
	if err != nil {
		t.Fatal(err)
	}
	stats := tester.Stats()
	if stats.Requests != 100 {
		t.Errorf("want 100 requests made, got %d", stats.Requests)
	}
	if stats.Successes != 100 {
		t.Errorf("want 100 successes, got %d", stats.Successes)
	}
	if stats.Failures != 0 {
		t.Errorf("want 0 failures, got %d", stats.Failures)
	}
	if stats.Requests != stats.Successes+stats.Failures {
		t.Error("want total requests to be the sum of successes + failures")
	}
	if tester.EndAt.Milliseconds() == 0 {
		t.Fatal("zero milliseconds is an invalid time")
	}
}

func TestTimeRecorderCalledMultipleTimesSetCorrectStatsAndReturnsNoError(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	tester.TimeRecorder.RecordTime(50)
	tester.TimeRecorder.RecordTime(100)
	tester.TimeRecorder.RecordTime(200)
	tester.TimeRecorder.RecordTime(100)
	tester.TimeRecorder.RecordTime(50)

	err = tester.SetMetrics()
	if err != nil {
		t.Fatal(err)
	}
	stats := tester.Stats()
	if stats.Mean != 100 {
		t.Errorf("want 100ms mean time, got %v", stats.Mean)
	}
}

func TestTimeRecorderCalledMultipleTimesSetCorrectPercentilesAndReturnsNoError(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	tester.TimeRecorder.RecordTime(5)
	tester.TimeRecorder.RecordTime(6)
	tester.TimeRecorder.RecordTime(7)
	tester.TimeRecorder.RecordTime(8)
	tester.TimeRecorder.RecordTime(10)
	tester.TimeRecorder.RecordTime(11)
	tester.TimeRecorder.RecordTime(13)

	err = tester.SetMetrics()
	if err != nil {
		t.Fatal(err)
	}
	stats := tester.Stats()
	if stats.P50 != 8 {
		t.Errorf("want 50th percentile request time of 8ms, got %v", stats.P50)
	}
	if stats.P90 != 11 {
		t.Errorf("want 90th percentile request time of 11ms, got %v", stats.P90)
	}
	if stats.P99 != 13 {
		t.Errorf("want 99th percentile request time of 13ms, got %v", stats.P99)
	}
}

func TestSetMetricsReturnsErrorIfRecordTimeIsNotCalled(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = tester.SetMetrics()
	if !errors.Is(err, bench.ErrTimeNotRecorded) {
		t.Errorf("want ErrTimeNotRecorded error if there is no ExecutionsTime, got %q", err)
	}
}

func TestLogPrintsToStdoutAndStderr(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStdout(stdout),
		bench.WithStderr(stderr),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "this message goes to stdout"
	tester.LogStdOut("this message goes to stdout")
	got := stdout.String()
	if want != got {
		t.Errorf("want message %q in stdout but found %q", want, got)
	}

	want = "this message goes to stderr"
	tester.LogStdErr("this message goes to stderr")
	got = stderr.String()
	if want != got {
		t.Errorf("want message %q in stderr but found %q", want, got)
	}
}

func TestLogfPrintsToStdoutAndStderr(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStdout(stdout),
		bench.WithStderr(stderr),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "this message goes to stdout"
	tester.LogFStdOut("this %s goes to %s", "message", "stdout")
	got := stdout.String()
	if want != got {
		t.Errorf("want message %q in stdout but found %q", want, got)
	}
	want = "this message goes to stderr"
	tester.LogFStdErr("this %s goes to %s", "message", "stderr")
	got = stderr.String()
	if want != got {
		t.Errorf("want message %q in stderr but found %q", want, got)
	}
}

func TestFromArgsNRequestsConfiguresNRequests(t *testing.T) {
	t.Parallel()
	args := []string{"run", "-r", "10", "-u", "http://fake.url"}
	tester, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs(args),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantReqs := 10
	gotReqs := tester.Requests()
	if wantReqs != gotReqs {
		t.Errorf("reqs: want %d, got %d", wantReqs, gotReqs)
	}
}

func TestNewTesterWithGraphsSetsGraphsMode(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStderr(io.Discard),
		bench.WithGraphs(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tester.Graphs {
		t.Error("want graphs to be true")
	}
}

func TestNewTesterWithNoGraphsSetsNoGraphsMode(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStderr(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	if tester.Graphs {
		t.Error("want graphs to be false")
	}
}

func TestFromArgsGraphsFlagConfiguresGraphsMode(t *testing.T) {
	t.Parallel()
	args := []string{"run", "-g", "-u", "http://fake.url"}
	tester, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs(args),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tester.Graphs {
		t.Error("want graphs to be true")
	}
}

func TestFromArgsNoGraphsFlagConfiguresNoGraphsMode(t *testing.T) {
	t.Parallel()
	args := []string{"run", "-u", "http://fake.url"}
	tester, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs(args),
	)
	if err != nil {
		t.Fatal(err)
	}
	if tester.Graphs {
		t.Error("want graphs to be false")
	}
}

func TestConfiguredGraphsFlagGenerateGraphs(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "HelloWorld")
	}))
	tempDir := t.TempDir()
	tester, err := bench.NewTester(
		bench.WithURL(server.URL),
		bench.WithHTTPClient(server.Client()),
		bench.WithStdout(io.Discard),
		bench.WithStderr(io.Discard),
		bench.WithOutputPath(tempDir),
	)
	if err != nil {
		t.Fatal(err)
	}
	tester.Graphs = true
	err = tester.Run()
	if err != nil {
		t.Fatal(err)
	}
	filePath := fmt.Sprintf("%s/%s", tester.OutputPath, "boxplot.png")
	_, err = os.Stat(filePath)
	if err != nil {
		t.Errorf("want file %q to exist", filePath)
	}
	filePath = fmt.Sprintf("%s/%s", tester.OutputPath, "histogram.png")
	_, err = os.Stat(filePath)
	if err != nil {
		t.Errorf("want file %q to exist", filePath)
	}
	t.Cleanup(func() {
		err := os.RemoveAll(tester.OutputPath)
		if err != nil {
			fmt.Printf("cannot delete %s\n", tester.OutputPath)
		}
	})
}

func TestUnconfiguredGraphsFlagDoesNotGenerateGraphs(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "HelloWorld")
	}))
	tempDir := t.TempDir()
	tester, err := bench.NewTester(
		bench.WithURL(server.URL),
		bench.WithHTTPClient(server.Client()),
		bench.WithStdout(io.Discard),
		bench.WithStderr(io.Discard),
		bench.WithOutputPath(tempDir),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = tester.Run()
	if err != nil {
		t.Fatal(err)
	}
	filePath := fmt.Sprintf("%s/%s", tester.OutputPath, "boxplot.png")
	_, err = os.Stat(filePath)
	if err == nil {
		t.Errorf("want file %q to not exist. Error found: %v", filePath, err)
	}
	filePath = fmt.Sprintf("%s/%s", tester.OutputPath, "histogram.png")
	_, err = os.Stat(filePath)
	if err == nil {
		t.Errorf("want file %q to not exist. Error found: %v", filePath, err)
	}
	t.Cleanup(func() {
		err := os.RemoveAll(tester.OutputPath)
		if err != nil {
			fmt.Printf("cannot delete %s\n", tester.OutputPath)
		}
	})
}

func TestNewTesterWithStatsSetsExportStatsMode(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStderr(io.Discard),
		bench.WithExportStats(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tester.ExportStats {
		t.Error("want ExportStats to be true")
	}
}

func TestNewTesterWithNoExportStatsSetsNoExportStatsMode(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStderr(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
	if tester.ExportStats {
		t.Error("want ExportStats to be false")
	}
}

func TestFromArgs_WithSFlagEnablesExportStatsMode(t *testing.T) {
	t.Parallel()
	args := []string{"run", "-s", "-u", "http://fake.url"}
	tester, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs(args),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !tester.ExportStats {
		t.Error("want ExportStats to be true")
	}
}

func TestFromArgs_WithoutSFlagDisablesExportStatsMode(t *testing.T) {
	t.Parallel()
	args := []string{"run", "-u", "http://fake.url"}
	tester, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs(args),
	)
	if err != nil {
		t.Fatal(err)
	}
	if tester.ExportStats {
		t.Error("want ExportStats to be false")
	}
}

func TestConfiguredExportStatsFlagGenerateStatsFile(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "HelloWorld")
	}))
	tempDir := t.TempDir()
	tester, err := bench.NewTester(
		bench.WithURL(server.URL),
		bench.WithHTTPClient(server.Client()),
		bench.WithStdout(io.Discard),
		bench.WithStderr(io.Discard),
		bench.WithOutputPath(tempDir),
	)
	if err != nil {
		t.Fatal(err)
	}
	tester.ExportStats = true
	err = tester.Run()
	if err != nil {
		t.Fatal(err)
	}
	filePath := fmt.Sprintf("%s/%s", tester.OutputPath, "statsfile.txt")
	_, err = os.Stat(filePath)
	if err != nil {
		t.Errorf("want file %q to exist", filePath)
	}

	t.Cleanup(func() {
		err := os.RemoveAll(tester.OutputPath)
		if err != nil {
			fmt.Printf("cannot delete %s\n", tester.OutputPath)
		}
	})
}

func TestUnconfiguredExportStatsFlagDoesNotGenerateStatsFile(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "HelloWorld")
	}))
	tempDir := t.TempDir()
	tester, err := bench.NewTester(
		bench.WithURL(server.URL),
		bench.WithHTTPClient(server.Client()),
		bench.WithStdout(io.Discard),
		bench.WithStderr(io.Discard),
		bench.WithOutputPath(tempDir),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = tester.Run()
	if err != nil {
		t.Fatal(err)
	}
	filePath := fmt.Sprintf("%s/%s", tester.OutputPath, "statsfile.txt")
	_, err = os.Stat(filePath)
	if err == nil {
		t.Errorf("want file %q to not exist. Error found: %v", filePath, err)
	}
	t.Cleanup(func() {
		err := os.RemoveAll(tester.OutputPath)
		if err != nil {
			fmt.Printf("cannot delete %s\n", tester.OutputPath)
		}
	})
}

func TestNewTesterWithURLSetsTesterURL(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "http://fake.url"
	if want != tester.URL {
		t.Fatalf("want tester URL %q, got %q", want, tester.URL)
	}
}

func TestFromArgsWithURLSetsTesterURL(t *testing.T) {
	t.Parallel()
	tester, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs([]string{"run", "-r", "10", "-u", "http://fake.url"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "http://fake.url"
	if want != tester.URL {
		t.Fatalf("want tester URL %q, got %q", want, tester.URL)
	}
}

func TestNewTesterWithoutWithURLReturnsErrorNoURL(t *testing.T) {
	t.Parallel()
	_, err := bench.NewTester(
		bench.WithStderr(io.Discard),
	)
	if !errors.Is(err, bench.ErrNoURL) {
		t.Errorf("want ErrNoURL error if no URL set, got %v", err)
	}
}

func TestFromArgs_GivenNoArgsReturnsUsageMessage(t *testing.T) {
	t.Parallel()
	_, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs([]string{}),
	)
	if !errors.Is(err, bench.ErrNoArgs) {
		t.Errorf("want ErrNoArgs error if no args supplied, got %v", err)
	}
}

func TestFromArgsWithoutURLReturnsErrorNoURL(t *testing.T) {
	t.Parallel()
	_, err := bench.NewTester(
		bench.WithStderr(io.Discard),
		bench.FromArgs([]string{"run", "-r", "10"}),
	)
	if !errors.Is(err, bench.ErrNoURL) {
		t.Errorf("want ErrNoURL error if no URL set, got %v", err)
	}
}

func TestNewTesterWithNilStdoutReturnsErrorValueCannotBeNil(t *testing.T) {
	t.Parallel()
	_, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStdout(nil),
	)
	if !errors.Is(err, bench.ErrValueCannotBeNil) {
		t.Errorf("want ErrValueCannotBeNil error if stdout is nil, got %v", err)
	}
}

func TestNewTesterWithNilStderrReturnsErrorValueCannotBeNil(t *testing.T) {
	t.Parallel()
	_, err := bench.NewTester(
		bench.WithURL("http://fake.url"),
		bench.WithStderr(nil),
	)
	if !errors.Is(err, bench.ErrValueCannotBeNil) {
		t.Errorf("want ErrValueCannotBeNil error if stderr is nil, got %v", err)
	}
}

func TestCompareStatsFiles_ReadsTwoFilesAndComparesThem(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f1 := dir + "/stats1.txt"
	stats1 := bench.Stats{
		Failures:  2,
		P50:       20,
		P90:       30,
		P99:       100,
		Requests:  20,
		Successes: 18,
	}
	f, err := os.Create(f1)
	if err != nil {
		t.Fatal(err)
	}
	err = bench.WriteStatsFile(f, stats1)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	stats2 := bench.Stats{
		Failures:  1,
		P50:       5,
		P90:       33,
		P99:       99,
		Requests:  40,
		Successes: 19,
	}
	f2 := dir + "/stats2.txt"
	f, err = os.Create(f2)
	if err != nil {
		t.Fatal(err)
	}
	err = bench.WriteStatsFile(f, stats2)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	got, err := bench.CompareStatsFiles(f1, f2)
	if err != nil {
		t.Fatal(err)
	}
	want := bench.StatsDelta{
		Failures:  -1,
		P50:       -15,
		P90:       3,
		P99:       -1,
		Requests:  20,
		Successes: 1,
	}
	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompareStatsFiles_ErrorsIfOneOrBothFilesUnreadable(t *testing.T) {
	_, err := bench.CompareStatsFiles("bogus", "even more bogus")
	if err == nil {
		t.Error("no error")
	}
}

func TestReadStatsFilePopulatesCorrectStats(t *testing.T) {
	t.Parallel()
	statsFileReader := strings.NewReader(`http://fake.url,20,19,1,100.123,150.000,198.465`)
	got, err := bench.ReadStatsFile(statsFileReader)
	if err != nil {
		t.Fatal(err)
	}
	want := []bench.Stats{
		{
			Failures:  1,
			P50:       100.123,
			P90:       150.000,
			P99:       198.465,
			Requests:  20,
			Successes: 19,
			URL:       "http://fake.url",
		},
	}
	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestCompareStatsReturnsCorrectStatsDelta(t *testing.T) {
	t.Parallel()
	stats1 := bench.Stats{
		Failures:  2,
		P50:       20,
		P90:       30,
		P99:       100,
		Requests:  20,
		Successes: 18,
	}
	stats2 := bench.Stats{
		Failures:  1,
		P50:       5,
		P90:       33,
		P99:       99,
		Requests:  40,
		Successes: 19,
	}
	got := bench.CompareStats(stats1, stats2)
	want := bench.StatsDelta{
		Failures:  -1,
		P50:       -15,
		P90:       3,
		P99:       -1,
		Requests:  20,
		Successes: 1,
	}
	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}

func TestWriteStatsFilePopulatesCorrectStatsFile(t *testing.T) {
	t.Parallel()
	stats := bench.Stats{
		Failures:  2,
		P50:       100.123,
		P90:       150.000,
		P99:       198.465,
		Requests:  20,
		Successes: 18,
		URL:       "http://fake.url",
	}
	output := &bytes.Buffer{}
	err := bench.WriteStatsFile(output, stats)
	if err != nil {
		t.Fatal(err)
	}
	want := `http://fake.url,20,18,2,100.123,150.000,198.465`
	got := output.String()
	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}
