package preprocess

import "testing"

func TestExtractBaseTool_Simple(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"nmap -sV 10.10.10.1", "nmap"},
		{"sudo nmap -sV 10.10.10.1", "nmap"},
		{"time gobuster dir -u http://target", "gobuster"},
		{"sudo -u root nohup nikto -h http://target", "nikto"},
		{"/usr/bin/git diff", "git"},
		{"ps aux", "ps"},
		{"", ""},
	}

	for _, tc := range cases {
		got := ExtractBaseTool(tc.cmd)
		if got != tc.want {
			t.Errorf("ExtractBaseTool(%q) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

func TestLookupExtractor_Registered(t *testing.T) {
	if ext := LookupExtractor("nmap -sV 10.10.10.1"); ext == nil {
		t.Error("expected extractor for nmap, got nil")
	}
	if ext := LookupExtractor("sudo gobuster dir -u http://x"); ext == nil {
		t.Error("expected extractor for gobuster, got nil")
	}
}

func TestLookupExtractor_Unregistered(t *testing.T) {
	if ext := LookupExtractor("wget http://example.com"); ext != nil {
		t.Error("expected nil extractor for wget")
	}
}

func TestExtractNmap(t *testing.T) {
	input := `Starting Nmap 7.94 ( https://nmap.org ) at 2024-01-01 12:00 UTC
Nmap scan report for 10.10.10.1
Host is up (0.03s latency).
Not shown: 997 closed ports
22/tcp   open  ssh     OpenSSH 8.9p1
80/tcp   open  http    Apache httpd 2.4.52
443/tcp  open  https   nginx 1.18.0
8080/tcp closed http-proxy
Service Info: OS: Linux; CPE: cpe:/o:linux:linux_kernel
Nmap done: 1 IP address (1 host up) scanned in 15.32 seconds`

	got := extractNmap(input)

	if !contains(got, "22/tcp") {
		t.Error("expected nmap output to contain 22/tcp")
	}
	if !contains(got, "80/tcp") {
		t.Error("expected nmap output to contain 80/tcp")
	}
	if contains(got, "8080/tcp") {
		t.Error("expected nmap output to NOT contain closed port 8080")
	}
	if !contains(got, "Nmap scan report for") {
		t.Error("expected nmap output to contain scan header")
	}
	if !contains(got, "Host is up") {
		t.Error("expected nmap output to contain host up line")
	}
	if !contains(got, "Service Info:") {
		t.Error("expected nmap output to contain Service Info")
	}
	if contains(got, "Not shown") {
		t.Error("expected nmap output to NOT contain 'Not shown' line")
	}
}

func TestExtractGobuster(t *testing.T) {
	input := `/index.html (Status: 200) [Size: 1234]
/admin (Status: 301) [Size: 567]
/secret (Status: 403) [Size: 89]
/nothing (Status: 404) [Size: 0]
/login (Status: 200) [Size: 890]`

	got := extractGobuster(input)

	if !contains(got, "/index.html") {
		t.Error("expected 200 result")
	}
	if !contains(got, "/admin") {
		t.Error("expected 301 result")
	}
	if !contains(got, "/secret") {
		t.Error("expected 403 result")
	}
	if contains(got, "/nothing") {
		t.Error("expected 404 result to be filtered out")
	}
}

func TestExtractPs(t *testing.T) {
	input := `USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.0  0.1 169312 11812 ?        Ss   Jan01   0:03 /sbin/init
www-data   456  0.1  0.5 234567 45678 ?        S    Jan01   1:23 /usr/sbin/apache2
user      1234  0.0  0.1 123456 12345 pts/0    Ss   12:00   0:00 bash
root       789  0.0  0.0      0     0 ?        S    Jan01   0:00 [kworker/0:0]`

	got := extractPs(input)

	if !contains(got, "USER") {
		t.Error("expected header line")
	}
	if !contains(got, "www-data") {
		t.Error("expected www-data process")
	}
	if !contains(got, "user") {
		t.Error("expected user process")
	}
	if !contains(got, "/sbin/init") {
		t.Error("expected root process to be kept (attack surface)")
	}
	if contains(got, "[kworker") {
		t.Error("expected kernel thread to be filtered")
	}
}

func TestExtractGit(t *testing.T) {
	input := `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main
+import "fmt"
 func main() {
-    println("hello")
+    fmt.Println("hello")
 }`

	got := extractGit(input)

	if !contains(got, "diff --git") {
		t.Error("expected diff header")
	}
	if !contains(got, "--- a/main.go") {
		t.Error("expected --- line")
	}
	if !contains(got, "+++ b/main.go") {
		t.Error("expected +++ line")
	}
	if !contains(got, "@@") {
		t.Error("expected @@ hunk header")
	}
	if contains(got, "println") {
		t.Error("expected actual diff content to be filtered")
	}
}

func TestExtractGit_NonDiff(t *testing.T) {
	input := `commit abc123
Author: User <user@example.com>
Date: Mon Jan 1 12:00:00 2024
    Initial commit`

	got := extractGit(input)
	if got != input {
		t.Error("expected git log output to pass through unchanged")
	}
}

func TestExtractNikto(t *testing.T) {
	input := `- Nikto v2.1.6
---------------------------------------------------------------------------
+ Target IP:          10.10.10.1
+ Target Hostname:    target
+ Target Port:        80
+ Server: Apache/2.4.52
+ The anti-clickjacking X-Frame-Options header is not present.
+ Allowed HTTP Methods: GET, HEAD, POST, OPTIONS
No error lines here.`

	got := extractNikto(input)

	if !contains(got, "Target IP") {
		t.Error("expected nikto finding line")
	}
	if contains(got, "- Nikto") {
		t.Error("expected non-finding line to be filtered")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsString(s, substr)
}

func containsString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLookupByContent_Nmap(t *testing.T) {
	nmapOutput := `Starting Nmap 7.94 ( https://nmap.org )
Nmap scan report for 10.10.10.1
Host is up (0.03s latency).
22/tcp   open  ssh     OpenSSH 8.9p1
80/tcp   open  http    Apache httpd 2.4.52
Nmap done: 1 IP address (1 host up) scanned in 15.32 seconds`

	ext, tool := LookupByContent(nmapOutput)
	if ext == nil {
		t.Fatal("expected extractor for nmap content, got nil")
	}
	if tool != "nmap" {
		t.Errorf("expected tool 'nmap', got %q", tool)
	}
	result := ext(nmapOutput)
	if !contains(result, "22/tcp") {
		t.Error("expected nmap extractor to keep open port 22")
	}
}

func TestLookupByContent_NoMatch(t *testing.T) {
	ext, tool := LookupByContent("just some regular text output\nnothing special here")
	if ext != nil {
		t.Error("expected nil extractor for generic content")
	}
	if tool != "" {
		t.Errorf("expected empty tool, got %q", tool)
	}
}

func TestPassthroughDetection_CatNmap(t *testing.T) {
	// LookupExtractor should return nil for "cat nmap.txt"
	ext := LookupExtractor("cat nmap.txt")
	if ext != nil {
		t.Error("expected nil extractor for 'cat' command")
	}

	// But PassthroughTools should recognize "cat"
	tool := ExtractBaseTool("cat nmap.txt")
	if !PassthroughTools[tool] {
		t.Errorf("expected 'cat' to be a passthrough tool, got tool=%q", tool)
	}

	// And LookupByContent should detect nmap output
	nmapOutput := `Nmap scan report for 10.10.10.1
22/tcp open ssh OpenSSH 8.9p1`
	contentExt, detectedTool := LookupByContent(nmapOutput)
	if contentExt == nil {
		t.Fatal("expected content-based extractor for nmap output")
	}
	if detectedTool != "nmap" {
		t.Errorf("expected detected tool 'nmap', got %q", detectedTool)
	}
}
