package trie

import (
	"encoding/json"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"
)

const (
	defaultROAsURL  = "https://hosted-routinator-east.rarc.net/json"
	localROAsPath   = "/var/tmp/routinator.json"
	fallbackTmpPath = "/tmp/routinator.json"
)

// ROAEntry represents an individual ROA record from Routinator JSON.
type ROAEntry struct {
	ASN       string `json:"asn"`
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"maxLength"`
	TA        string `json:"ta"`
}

// loadROAs retrieves ROA prefix strings from a local cache file or downloads from the URL.
func loadROAs(t testing.TB) []string {
	// 1. Try local cached paths
	for _, path := range []string{localROAsPath, fallbackTmpPath} {
		if _, err := os.Stat(path); err == nil {
			prefixes, err := parseROAFile(path)
			if err == nil && len(prefixes) > 0 {
				t.Logf("Loaded %d ROA prefixes from local file %s", len(prefixes), path)
				return prefixes
			}
		}
	}

	// 2. Fetch from remote URL
	t.Logf("Downloading ROA data from %s ...", defaultROAsURL)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(defaultROAsURL)
	if err != nil {
		t.Skipf("Skipping scale test: unable to download ROA data: %v", err)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping scale test: remote server returned status %v", resp.Status)
		return nil
	}

	// Cache to fallback file
	out, err := os.Create(fallbackTmpPath)
	if err == nil {
		_, _ = io.Copy(out, resp.Body)
		_ = out.Close()
		prefixes, err := parseROAFile(fallbackTmpPath)
		if err == nil {
			return prefixes
		}
	}

	t.Skip("Skipping scale test: failed to cache and parse ROA data")
	return nil
}

// parseROAFile streams the ROA JSON file to extract all prefix strings efficiently.
func parseROAFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var prefixes []string
	dec := json.NewDecoder(file)

	// Stream until we reach the "roas" token
	for {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if str, ok := t.(string); ok && str == "roas" {
			break
		}
	}

	// Read opening bracket of the "roas" array '['
	_, err = dec.Token()
	if err != nil {
		return nil, err
	}

	for dec.More() {
		var entry ROAEntry
		if err := dec.Decode(&entry); err != nil {
			continue
		}
		if entry.Prefix != "" {
			prefixes = append(prefixes, entry.Prefix)
		}
	}

	return prefixes, nil
}

// generateHostIP extracts or generates a host IP address within the given prefix.
func generateHostIP(cidr string) net.IP {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}

	if ip4 := ip.To4(); ip4 != nil {
		host := make(net.IP, 4)
		copy(host, ip4)
		// Change last octet or host portion
		ones, bits := ipnet.Mask.Size()
		if bits == 32 && ones < 32 {
			host[3] |= 0x01
		}
		return host
	}

	if ip6 := ip.To16(); ip6 != nil {
		host := make(net.IP, 16)
		copy(host, ip6)
		host[15] |= 0x01
		return host
	}

	return nil
}

func TestScaleInternetRoutes(t *testing.T) {
	prefixes := loadROAs(t)
	if len(prefixes) == 0 {
		t.Skip("No ROA prefixes loaded")
		return
	}

	t.Logf("=== Starting Scale Test with %d Prefixes ===", len(prefixes))

	// Track Memory
	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// 1. Benchmark Insertion of ~990k prefixes
	tree, err := New()
	if err != nil {
		t.Fatalf("Failed to initialize trie: %v", err)
	}

	startInsert := time.Now()
	var insertedCount int
	for _, p := range prefixes {
		if err := tree.InsertCIDR(p); err == nil {
			insertedCount++
		}
	}
	insertDuration := time.Since(startInsert)

	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	heapAllocMB := float64(memAfter.Alloc-memBefore.Alloc) / (1024 * 1024)
	if heapAllocMB < 0 {
		heapAllocMB = float64(memAfter.Alloc) / (1024 * 1024)
	}
	bytesPerPrefix := float64(memAfter.Alloc-memBefore.Alloc) / float64(tree.Size())
	avgInsertTime := insertDuration / time.Duration(len(prefixes))
	insertThroughput := float64(len(prefixes)) / insertDuration.Seconds()

	t.Logf("--- Insertion Results ---")
	t.Logf("Total Prefixes Processed:  %d", len(prefixes))
	t.Logf("Unique Prefixes in Trie:   %d", tree.Size())
	t.Logf("Total Insertion Time:      %v", insertDuration)
	t.Logf("Avg Time Per Insertion:    %v", avgInsertTime)
	t.Logf("Insertion Throughput:      %.2f prefixes/sec", insertThroughput)
	t.Logf("Heap Allocated Memory:     %.2f MB (approx. %.1f bytes/prefix)", heapAllocMB, bytesPerPrefix)

	// 2. Perform 1000 Lookups of host addresses
	const numLookups = 1000
	r := rand.New(rand.NewSource(42)) // Deterministic seed for reproducibility

	sampleIPs := make([]net.IP, 0, numLookups)
	for i := 0; i < numLookups; i++ {
		idx := r.Intn(len(prefixes))
		hostIP := generateHostIP(prefixes[idx])
		if hostIP != nil {
			sampleIPs = append(sampleIPs, hostIP)
		}
	}

	t.Logf("--- Starting %d Host Lookups ---", len(sampleIPs))

	lookupDurations := make([]time.Duration, len(sampleIPs))
	var matchedCount int

	startTotalLookup := time.Now()
	for i, ip := range sampleIPs {
		start := time.Now()
		matchedNet, err := tree.Lpm(ip)
		dur := time.Since(start)
		lookupDurations[i] = dur
		if err == nil && matchedNet != nil {
			matchedCount++
		}
	}
	totalLookupDuration := time.Since(startTotalLookup)

	// 3. Compute Min / Max / Average / Percentiles
	var minDur, maxDur, sumDur time.Duration
	minDur = time.Duration(math.MaxInt64)

	for _, dur := range lookupDurations {
		if dur < minDur {
			minDur = dur
		}
		if dur > maxDur {
			maxDur = dur
		}
		sumDur += dur
	}

	avgDur := sumDur / time.Duration(len(lookupDurations))

	// Sort for percentiles
	sort.Slice(lookupDurations, func(i, j int) bool {
		return lookupDurations[i] < lookupDurations[j]
	})

	p50 := lookupDurations[len(lookupDurations)*50/100]
	p95 := lookupDurations[len(lookupDurations)*95/100]
	p99 := lookupDurations[len(lookupDurations)*99/100]
	lookupThroughput := float64(len(sampleIPs)) / totalLookupDuration.Seconds()

	t.Logf("--- Lookup Results (%d lookups, %d matched) ---", len(sampleIPs), matchedCount)
	t.Logf("Total Time for %d Lookups: %v", len(sampleIPs), totalLookupDuration)
	t.Logf("Min Lookup Time:           %v", minDur)
	t.Logf("Max Lookup Time:           %v", maxDur)
	t.Logf("Average (Mean) Time:       %v", avgDur)
	t.Logf("Median (p50) Time:         %v", p50)
	t.Logf("95th Percentile (p95):     %v", p95)
	t.Logf("99th Percentile (p99):     %v", p99)
	t.Logf("Lookup Throughput:         %.2f queries/sec", lookupThroughput)
}

func BenchmarkScaleInsertion(b *testing.B) {
	prefixes := loadROAs(b)
	if len(prefixes) == 0 {
		b.Skip("No ROA prefixes loaded")
		return
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree, err := New()
		if err != nil {
			b.Fatalf("New() failed: %v", err)
		}
		for _, p := range prefixes {
			_ = tree.InsertCIDR(p)
		}
	}
}

func BenchmarkScaleLookups(b *testing.B) {
	prefixes := loadROAs(b)
	if len(prefixes) == 0 {
		b.Skip("No ROA prefixes loaded")
		return
	}

	tree, err := New()
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	for _, p := range prefixes {
		_ = tree.InsertCIDR(p)
	}

	r := rand.New(rand.NewSource(12345))
	const poolSize = 1000
	testIPs := make([]net.IP, poolSize)
	for i := 0; i < poolSize; i++ {
		testIPs[i] = generateHostIP(prefixes[r.Intn(len(prefixes))])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := testIPs[i%poolSize]
		_, _ = tree.Lpm(ip)
	}
}
