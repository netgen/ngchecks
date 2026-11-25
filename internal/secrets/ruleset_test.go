package secrets_test

import (
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/netgen/ngchecks/internal/secrets"
	"github.com/netgen/ngchecks/internal/testutil"
	"github.com/netgen/ngchecks/ruleset"
)

func TestRulesetIsValid(t *testing.T) {
	binary := testutil.Betterleaks(t)
	path := t.TempDir() + "/ruleset.toml"
	testutil.WriteFile(t, path, string(ruleset.TOML))

	output, err := exec.Command(binary, "config", "check", path, "--no-banner").CombinedOutput()
	if err != nil {
		t.Fatalf("invalid ruleset: %v\n%s", err, output)
	}
}

// The ruleset's global prefilter and filter replace the built-in ones, so they must contain
// them. Fails after a Betterleaks upgrade that changed the defaults.
func TestRulesetContainsBuiltInGlobalExpressions(t *testing.T) {
	output, err := exec.Command(testutil.Betterleaks(t), "config", "show", "--no-banner").Output()
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"prefilter", "filter"} {
		match := regexp.MustCompile(`(?ms)^` + key + ` = '''\n(.*?)'''`).FindSubmatch(output)
		if match == nil {
			t.Fatalf("no built-in %s found", key)
		}

		if !strings.Contains(string(ruleset.TOML), strings.TrimSpace(string(match[1]))) {
			t.Errorf("the built-in %s changed, update the copy in ruleset/betterleaks.toml:\n%s", key, match[1])
		}
	}
}

func TestRulesetDetectsLeaks(t *testing.T) {
	tests := []struct {
		name, rule, file, content string
	}{
		{"Symfony vault decryption key file", "symfony-secrets-decrypt-key", "config/secrets/prod/prod.decrypt.private.php", "<?php return 'key';\n"},
		{"Symfony vault decryption key variable", "symfony-decryption-secret", "compose.yaml", "environment:\n  SYMFONY_DECRYPTION_SECRET: q7N2xV9kLp3Rt8Zm4Wc6Yb1Hd5Fg0Js2Ka9Ue7Io3Pn6Ql8Mr4Ts1Vw5Xy\n"},
		{"committed .env.local", "dotenv-local-file", ".env.local", "APP_DEBUG=1\n"},
		{"committed .env.development.local", "dotenv-local-file", ".env.development.local", "NEXT_PUBLIC_DEBUG=1\n"},
		{"APP_SECRET in .env", "dotenv-secret", ".env", "APP_SECRET=7f3d9a1c2b4e6f8a0d1c3e5f7a9b1d2e\n"},
		{"password variable in .env.prod", "dotenv-secret", ".env.prod", "export SMTP_PASSWORD='Hq7vR2xK9mPz'\n"},
		{"database URL with password", "dsn-credentials", ".env", "DATABASE_URL=\"postgresql://ngadmin:Hq7vR2xK9mPz@db.example.com:5432/app\"\n"},
		{"Redis DSN with password only", "dsn-credentials", "config/packages/cache.yaml", "framework:\n  cache:\n    default_redis_provider: 'redis://:Zr8Tq2Lm5Wx9@redis:6379'\n"},
		{"repository URL with credentials", "dsn-credentials", "composer.json", `{"repositories": [{"type": "composer", "url": "https://deploy:Qw8Er2Ty6Ui0@packages.example.com"}]}` + "\n"},
		{"mailer DSN with API key", "symfony-mailer-dsn-api-key", ".env", "MAILER_DSN=sendgrid+api://SG.k2Jd9LmQp4Xz7RtV1bNc@default\n"},
		{"Cloudinary URL", "cloudinary-url", ".env", "CLOUDINARY_URL=cloudinary://123456789012345:Ab3dEf6hIj9kLm2nOp5qRs8tUv@example\n"},
		{"Composer auth.json", "composer-auth-credentials", "auth.json", `{"http-basic": {"updates.ibexa.co": {"username": "0123456789", "password": "9a8b7c6d5e4f3a2b1c0d"}}}` + "\n"},
		{"COMPOSER_AUTH in CI config", "composer-auth-env-credentials", ".github/workflows/ci.yml", "env:\n  COMPOSER_AUTH: '{\"http-basic\":{\"repo.example.com\":{\"username\":\"u\",\"password\":\"Tz8Kq3Vm6Np1\"}}}'\n"},
		{"Deployer password", "deployer-secret", "deploy/hosts.php", "<?php\nhost('prod')->set('become_password', 'Kx9mQ2vL7pRz');\n"},
		{"SSH private key file", "private-key-file", ".ssh/id_rsa", "not really a key\n"},
		{"npm registry token", "npm-registry-auth", ".npmrc", "//npm.pkg.example.com/:_authToken=Kq8Zt3Vn6Rm1Wx4Yp7Lc\n"},
		{"Yarn registry token", "npm-registry-auth", ".yarnrc.yml", "npmScopes:\n  acme:\n    npmAuthToken: Kq8Zt3Vn6Rm1Wx4Yp7Lc\n"},
		{"GitHub token (built-in rule)", "github-pat", "src/client.js", "const token = '" + testutil.FakeGitHubToken() + "';\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := testutil.NewRepo(t)
			repo.Write(test.file, test.content).Commit("Leak")

			if rules := scanRules(t, repo); !slices.Contains(rules, test.rule) {
				t.Errorf("expected rule %s, got %v", test.rule, rules)
			}
		})
	}
}

func TestRulesetIgnoresNonLeaks(t *testing.T) {
	tests := []struct {
		name, file, content string
	}{
		{"Symfony recipe .env", ".env", `APP_ENV=dev
APP_SECRET=
DATABASE_URL="postgresql://app:!ChangeMe!@127.0.0.1:5432/app?serverVersion=16&charset=utf8"
MESSENGER_TRANSPORT_DSN=doctrine://default?auto_setup=0
MAILER_DSN=null://null
LOCK_DSN=flock
JWT_SECRET_KEY=%kernel.project_dir%/config/jwt/private.pem
JWT_PUBLIC_KEY=%kernel.project_dir%/config/jwt/public.pem
JWT_PASSPHRASE=
JWT_TOKEN_TTL=3600
`},
		{"Symfony recipe .env.test", ".env.test", "KERNEL_CLASS='App\\Kernel'\nAPP_SECRET='$ecretf0rt3st'\n"},
		{"local development database", ".env", "DATABASE_URL=\"mysql://root:root@mysql:3306/app\"\n"},
		{"mailer DSN with env placeholder", ".env", "MAILER_DSN=mailgun+api://%env(MAILGUN_KEY)%:mg.example.com@default\n"},
		{"secret from env var processor", "config/packages/framework.yaml", "framework:\n    secret: '%env(APP_SECRET)%'\n"},
		{"Deployer without hardcoded secrets", "deploy.php", "<?php\nset('http_user', 'www-data');\nset('db_password', getenv('DB_PASSWORD'));\n"},
		{"COMPOSER_AUTH from CI secret", ".github/workflows/ci.yml", "env:\n  COMPOSER_AUTH: '{\"http-basic\":{\"repo.example.com\":{\"username\":\"u\",\"password\":\"${{ secrets.PW }}\"}}}'\n"},
		{"npm token from environment", ".npmrc", "//registry.npmjs.org/:_authToken=${NPM_TOKEN}\n"},
		{"committed vendor directory", "vendor/acme/lib/tests/fixtures.php", "<?php\n$token = '" + testutil.FakeGitHubToken() + "';\n"},
		{"built assets", "public/build/app.js", "const token = '" + testutil.FakeGitHubToken() + "';\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := testutil.NewRepo(t)
			repo.Write(test.file, test.content).Commit("No leak")

			if rules := scanRules(t, repo); len(rules) != 0 {
				t.Errorf("expected no findings, got %v", rules)
			}
		})
	}
}

func scanRules(t *testing.T, repo *testutil.Repo) []string {
	t.Helper()

	findings, err := secrets.Scanner{Binary: testutil.Betterleaks(t), Ruleset: ruleset.TOML}.Scan(repo.Root, secrets.Target{Mode: secrets.TrackedFiles})
	if err != nil {
		t.Fatal(err)
	}

	var rules []string
	for _, finding := range findings {
		rules = append(rules, finding.RuleID)
	}

	return rules
}
