package bench

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

const (
	DefaultConcurrency = 1
	DefaultNumRequests = 1
	DefaultOutputPath  = "./"
	DefaultUserAgent   = "Bench 0.0.1 Alpha"
)

var (
	DefaultHTTPClient = &http.Client{
		Timeout: 5 * time.Second,
	}
	ErrNoArgs           = errors.New("no arguments")
	ErrNoURL            = errors.New("no URL to test")
	ErrTimeNotRecorded  = errors.New("no execution time recorded")
	ErrValueCannotBeNil = errors.New("value cannot be nil")
)

type Tester struct {
	Concurrency    int
	client         *http.Client
	EndAt          time.Duration
	ExportStats    bool
	Graphs         bool
	OutputPath     string
	requests       int
	startAt        time.Time
	stdout, stderr io.Writer
	URL            string
	userAgent      string
	wg             *sync.WaitGroup
	Work           chan struct{}

	mu           *sync.Mutex
	stats        Stats
	TimeRecorder TimeRecorder
}

func NewTester(opts ...Option) (*Tester, error) {
	tester := &Tester{
		client:      DefaultHTTPClient,
		Concurrency: DefaultConcurrency,
		OutputPath:  DefaultOutputPath,
		requests:    DefaultNumRequests,
		stats:       Stats{},
		stderr:      os.Stderr,
		stdout:      os.Stdout,
		TimeRecorder: TimeRecorder{
			ExecutionsTime: []float64{},
			mu:             &sync.Mutex{},
		},
		userAgent: DefaultUserAgent,
		wg:        &sync.WaitGroup{},
		mu:        &sync.Mutex{},
	}
	for _, o := range opts {
		err := o(tester)
		if err != nil {
			return nil, err
		}
	}
	if tester.URL == "" {
		return nil, ErrNoURL
	}
	u, err := url.Parse(tester.URL)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid URL %q", u)
	}
	if tester.requests < 1 {
		return nil, fmt.Errorf("%d is invalid number of requests", tester.requests)
	}
	tester.Work = make(chan struct{})
	return tester, nil
}

func FromArgs(args []string) Option {
	return func(t *Tester) error {
		fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		fs.SetOutput(t.stderr)
		reqs := fs.Int("r", 1, "number of requests to be performed in the benchmark")
		graphs := fs.Bool("g", false, "generate graphs")
		exportStats := fs.Bool("s", false, "generate stats file")
		concurrency := fs.Int("c", 1, "number of concurrent requests (users) to run benchmark")
		url := fs.String("u", "", "url to run benchmark")
		if len(args) < 1 {
			fs.Usage()
			return ErrNoArgs
		}
		switch args[0] {
		case "run":
			fs.Parse(args[1:])
			t.URL = *url
			t.requests = *reqs
			t.Graphs = *graphs
			t.Concurrency = *concurrency
			t.ExportStats = *exportStats
		default:
			return errors.New("expected run or cmp subcommands")
		}
		return nil
	}
}

func WithRequests(reqs int) Option {
	return func(t *Tester) error {
		t.requests = reqs
		return nil
	}
}

func WithHTTPUserAgent(userAgent string) Option {
	return func(t *Tester) error {
		t.userAgent = userAgent
		return nil
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(t *Tester) error {
		t.client = client
		return nil
	}
}

func WithStdout(w io.Writer) Option {
	return func(t *Tester) error {
		if w == nil {
			return ErrValueCannotBeNil
		}
		t.stdout = w
		return nil
	}
}

func WithStderr(w io.Writer) Option {
	return func(lg *Tester) error {
		if w == nil {
			return ErrValueCannotBeNil
		}
		lg.stderr = w
		return nil
	}
}

func WithConcurrency(c int) Option {
	return func(lg *Tester) error {
		lg.Concurrency = c
		return nil
	}
}

func WithURL(URL string) Option {
	return func(t *Tester) error {
		t.URL = URL
		return nil
	}
}

func WithOutputPath(outputPath string) Option {
	return func(t *Tester) error {
		t.OutputPath = outputPath
		return nil
	}
}

func WithGraphs(graphs bool) Option {
	return func(t *Tester) error {
		t.Graphs = graphs
		return nil
	}
}

func WithExportStats(exportStats bool) Option {
	return func(t *Tester) error {
		t.ExportStats = exportStats
		return nil
	}
}

func (t Tester) HTTPUserAgent() string {
	return t.userAgent
}

func (t Tester) HTTPClient() *http.Client {
	return t.client
}

func (t Tester) StartTime() time.Time {
	return t.startAt
}

func (t Tester) Stats() Stats {
	return t.stats
}

func (t Tester) Requests() int {
	return t.requests
}

func (t *Tester) DoRequest() {
	for range t.Work {
		t.RecordRequest()
		req, err := http.NewRequest(http.MethodGet, t.URL, nil)
		if err != nil {
			t.LogStdErr(err.Error())
			t.RecordFailure()
			return
		}
		req.Header.Set("user-agent", t.HTTPUserAgent())
		req.Header.Set("accept", "*/*")
		startTime := time.Now()
		resp, err := t.client.Do(req)
		elapsedTime := time.Since(startTime)
		if err != nil {
			t.RecordFailure()
			t.LogStdErr(err.Error())
			return
		}
		t.TimeRecorder.RecordTime(float64(elapsedTime.Nanoseconds()) / 1000000.0)
		if resp.StatusCode != http.StatusOK {
			t.LogFStdErr("unexpected status code %d\n", resp.StatusCode)
			t.RecordFailure()
			return
		}
		t.RecordSuccess()
	}
}

func (t *Tester) Run() error {
	t.wg.Add(t.Concurrency)
	go func() {
		for x := 0; x < t.requests; x++ {
			t.Work <- struct{}{}
		}
		close(t.Work)
	}()
	t.startAt = time.Now()
	go func() {
		for x := 0; x < t.Concurrency; x++ {
			go func() {
				t.DoRequest()
				t.wg.Done()
			}()
		}
	}()
	t.wg.Wait()
	t.EndAt = time.Since(t.startAt)
	err := t.SetMetrics()
	if err != nil {
		return err
	}
	if t.Graphs {
		err = t.Boxplot()
		if err != nil {
			return err
		}
		err = t.Histogram()
		if err != nil {
			return err
		}
	}
	if t.ExportStats {
		file, err := os.Create(fmt.Sprintf("%s/%s", t.OutputPath, "statsfile.txt"))
		if err != nil {
			return err
		}
		defer file.Close()
		err = WriteStatsFile(file, t.Stats())
		if err != nil {
			return err
		}
	}
	t.LogFStdOut("The benchmark of %s site took %v\n", t.URL, t.EndAt.Round(time.Millisecond))
	t.LogFStdOut("Requests: %d Success: %d Failures: %d\n", t.stats.Requests, t.stats.Successes, t.stats.Failures)
	t.LogFStdOut("P50: %.3fms P90: %.3fms P99: %.3fms\n", t.stats.P50, t.stats.P90, t.stats.P99)
	return nil
}

func (t Tester) Boxplot() error {
	p := plot.New()
	p.Title.Text = "Latency boxplot"
	p.Y.Label.Text = "latency (ms)"
	p.X.Label.Text = t.URL
	w := vg.Points(20)
	box, err := plotter.NewBoxPlot(w, 0, plotter.Values(t.TimeRecorder.ExecutionsTime))
	if err != nil {
		return err
	}
	p.Add(box)
	err = p.Save(600, 400, fmt.Sprintf("%s/%s", t.OutputPath, "boxplot.png"))
	if err != nil {
		return err
	}
	return nil
}

func (t Tester) Histogram() error {
	p := plot.New()
	p.Title.Text = "Latency Histogram"
	p.Y.Label.Text = "n reqs"
	p.X.Label.Text = "latency (ms)"
	hist, err := plotter.NewHist(plotter.Values(t.TimeRecorder.ExecutionsTime), 50)
	if err != nil {
		return err
	}
	p.Add(hist)
	err = p.Save(600, 400, fmt.Sprintf("%s/%s", t.OutputPath, "histogram.png"))
	if err != nil {
		return err
	}
	return nil
}

func (t *Tester) RecordRequest() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.Requests++
}

func (t *Tester) RecordSuccess() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.Successes++
}

func (t *Tester) RecordFailure() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.Failures++
}

func (t Tester) LogStdOut(msg string) {
	fmt.Fprint(t.stdout, msg)
}

func (t Tester) LogStdErr(msg string) {
	fmt.Fprint(t.stderr, msg)
}

func (t Tester) LogFStdOut(msg string, opts ...interface{}) {
	fmt.Fprintf(t.stdout, msg, opts...)
}

func (t Tester) LogFStdErr(msg string, opts ...interface{}) {
	fmt.Fprintf(t.stderr, msg, opts...)
}

func (t *Tester) SetMetrics() error {
	times := t.TimeRecorder.ExecutionsTime
	if len(times) < 1 {
		return ErrTimeNotRecorded
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})
	p50Idx := int(math.Round(float64(len(times))*0.5)) - 1
	t.stats.P50 = times[p50Idx]
	p90Idx := int(math.Round(float64(len(times))*0.9)) - 1
	t.stats.P90 = times[p90Idx]
	p99Idx := int(math.Round(float64(len(times))*0.99)) - 1
	t.stats.P99 = times[p99Idx]

	nreq := 0.0
	totalTime := 0.0
	for _, v := range times {
		nreq++
		totalTime += v
	}
	t.stats.URL = t.URL
	t.stats.Mean = totalTime / nreq
	return nil
}

type Stats struct {
	URL       string
	Mean      float64
	P50       float64
	P90       float64
	P99       float64
	Failures  int
	Requests  int
	Successes int
}

type StatsDelta struct {
	P50       float64
	P90       float64
	P99       float64
	Requests  int
	Failures  int
	Successes int
}

type TimeRecorder struct {
	mu             *sync.Mutex
	ExecutionsTime []float64
}

func (t *TimeRecorder) RecordTime(executionTime float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ExecutionsTime = append(t.ExecutionsTime, executionTime)
}

type Option func(*Tester) error

func CompareStats(stats1, stats2 Stats) StatsDelta {
	statsDelta := StatsDelta{
		P50:       stats2.P50 - stats1.P50,
		P90:       stats2.P90 - stats1.P90,
		P99:       stats2.P99 - stats1.P99,
		Requests:  stats2.Requests - stats1.Requests,
		Successes: stats2.Successes - stats1.Successes,
		Failures:  stats2.Failures - stats1.Failures,
	}
	return statsDelta
}

func CompareStatsFiles(path1, path2 string) (StatsDelta, error) {
	f1, err := os.Open(path1)
	if err != nil {
		return StatsDelta{}, err
	}
	defer f1.Close()
	ReadStatsFile(f1)
	f2, err := os.Open(path1)
	if err != nil {
		return StatsDelta{}, err
	}
	defer f2.Close()
	return StatsDelta{}, nil
}

func ReadStatsFile(r io.Reader) ([]Stats, error) {
	scanner := bufio.NewScanner(r)
	stats := []Stats{}
	for scanner.Scan() {
		pos := strings.Split(scanner.Text(), ",")
		url := pos[0]
		dataRequests := pos[1]
		requests, err := strconv.Atoi(dataRequests)
		if err != nil {
			return nil, err
		}
		dataSuccesses := pos[2]
		successes, err := strconv.Atoi(dataSuccesses)
		if err != nil {
			return nil, err
		}
		dataFailures := pos[3]
		failures, err := strconv.Atoi(dataFailures)
		if err != nil {
			return nil, err
		}
		dataP50 := pos[4]
		p50, err := strconv.ParseFloat(dataP50, 64)
		if err != nil {
			return nil, err
		}
		dataP90 := pos[5]
		p90, err := strconv.ParseFloat(dataP90, 64)
		if err != nil {
			return nil, err
		}
		dataP99 := pos[6]
		p99, err := strconv.ParseFloat(dataP99, 64)
		if err != nil {
			return nil, err
		}
		stats = append(stats, Stats{
			Failures:  failures,
			P50:       p50,
			P90:       p90,
			P99:       p99,
			Requests:  requests,
			Successes: successes,
			URL:       url,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func WriteStatsFile(w io.Writer, stats Stats) error {
	_, err := fmt.Fprintf(w, "%s,%d,%d,%d,%.3f,%.3f,%.3f",
		stats.URL, stats.Requests, stats.Successes, stats.Failures, stats.P50, stats.P90, stats.P99,
	)
	if err != nil {
		return err
	}
	return nil
}
