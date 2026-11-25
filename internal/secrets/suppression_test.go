package secrets_test

import (
	"slices"
	"testing"

	"github.com/netgen/ngchecks/internal/secrets"
)

func TestSuppressionsFromDiff(t *testing.T) {
	diff := `diff --git a/.betterleaksignore b/.betterleaksignore
new file mode 100644
--- /dev/null
+++ b/.betterleaksignore
@@ -0,0 +1,3 @@
+# accepted
+abc123:config/app.php:generic-api-key:12
+
diff --git a/.betterleaks.toml b/.betterleaks.toml
--- a/.betterleaks.toml
+++ b/.betterleaks.toml
@@ -2,0 +3,2 @@
+  ` + "`^tests/`" + `,
+  ` + "`^fixtures/`" + `
diff --git a/src/Client.php b/src/Client.php
--- a/src/Client.php
+++ b/src/Client.php
@@ -10 +10 @@
-    $token = getenv('TOKEN');
+    $token = 'abc'; // Betterleaks:allow
@@ -20,0 +21,2 @@
+    // a normal line
+    $other = 'def'; # gitleaks:allow
diff --git a/removed.php b/removed.php
deleted file mode 100644
--- a/removed.php
+++ /dev/null
@@ -1 +0,0 @@
-$x = 1; // betterleaks:allow
`

	expected := []secrets.Suppression{
		{".betterleaksignore", 2, "ignore file entry", "abc123:config/app.php:generic-api-key:12"},
		{".betterleaks.toml", 3, "project config changed", "`^tests/`,"},
		{"src/Client.php", 10, "inline allow comment", "$token = 'abc'; // Betterleaks:allow"},
		{"src/Client.php", 22, "inline allow comment", "$other = 'def'; # gitleaks:allow"},
	}

	if got := secrets.SuppressionsFromDiff(diff); !slices.Equal(got, expected) {
		t.Errorf("expected\n%v\ngot\n%v", expected, got)
	}
}
