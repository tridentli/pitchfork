# Pitchfork Codebase Modernization & Security Review Plan

This document outlines a phased plan to modernize the Pitchfork framework, improve its testability and coverage, and conduct a thorough security review.

---

## Executive Summary

Pitchfork is a modular and secure web application and CLI framework written in Go. While it has a solid architectural foundation (Unified Execution Engine, strict database auditing, proxy-safe IP resolution), it currently faces several technical debt challenges:
*   **Outdated Go Version:** Currently using Go 1.18 (released in early 2022).
*   **Outdated Dependencies:** Many third-party libraries are outdated or deprecated.
*   **High Barrier to Testing:** Existing tests are integration tests requiring an active PostgreSQL database and specific environment variables, resulting in 0% actual unit test coverage during standard `go test` runs.
*   **Custom Components:** Relying on custom reflection-based ORM and Forms compilers which are hard to maintain and audit.

This plan proposes a **5-Phase approach** to address these issues, targeting modern Go standards, near 100% test coverage for critical components, and a robust security posture.

---

## Phase 1: Assessment & Local Test Environment Setup

**Goal:** Enable reliable, repeatable execution of existing integration tests locally and in CI.

### Actions:
1.  **Create a Containerized Test Environment:**
    *   Develop a `docker-compose.yml` file to spin up a local PostgreSQL database.
    *   Define a Dockerfile for Pitchfork that can run migrations and execute tests.
2.  **Document and Automate Test Execution:**
    *   Create a script (e.g., `scripts/run_tests.sh`) that automatically sets up the database, applies schema migrations (`share/dbschemas/`), sets required environment variables (`PITCHFORK_TOOLNAME`, `PITCHFORK_CONFROOT`), and runs the tests.
    *   Fix the early-exit behavior in `lib/test/helpers.go` so that tests fail visibly if the environment is misconfigured, rather than silently passing with 0% coverage.
3.  **Baseline Coverage Assessment:**
    *   Run the existing tests in the containerized environment to get a true baseline coverage report.

---

## Phase 2: Go & Dependency Modernization

**Goal:** Upgrade the core Go runtime and modernize third-party dependencies.

### Actions:
1.  **Upgrade Go Version:**
    *   Upgrade `go.mod` to **Go 1.21** (or **1.22**).
    *   Update `debian/control` to reflect the newer Go version requirement for packaging.
2.  **Update Key Dependencies:**
    *   `github.com/golang-jwt/jwt`: Upgrade to `github.com/golang-jwt/jwt/v5` (or v4) as v3 is deprecated and has known security issues.
    *   `github.com/russross/blackfriday`: Upgrade to v2 or migrate to `github.com/yuin/goldmark` (the standard markdown parser in modern Go).
    *   `github.com/nicksnyder/go-i18n`: Upgrade to v2 for better localization support.
    *   `github.com/pborman/uuid`: Replace with the more standard `github.com/google/uuid`.
3.  **Refactor for Modern Go Features:**
    *   **Structured Logging (`slog`):** Evaluate replacing custom logging in `lib/db.go` and `lib/misc.go` with the standard library's `charcoal/slog` (available in Go 1.21+) for structured, level-based logging.
    *   **Generics:** Explore using generics to simplify the custom ORM (`lib/struct.go`) or other utility functions.

---

## Phase 3: Test Suite Refactoring & Decoupling (Unit Testing)

**Goal:** Decouple business logic from the database to allow fast, hermetic unit tests, aiming for high coverage.

### Actions:
1.  **Introduce Database Mocking:**
    *   Integrate `github.com/DATA-DOG/go-sqlmock` into the test suite.
    *   Refactor `PfDB` connection logic to allow injecting a mocked `*sql.DB` connection during tests.
2.  **Refactor to Interfaces (Dependency Injection):**
    *   Define interfaces for core services (e.g., User Service, Group Service, Wiki Service).
    *   Modify the handlers and commands to accept these interfaces rather than relying strictly on global state. This allows mocking services in tests.
3.  **Write Comprehensive Unit Tests:**
    *   **Priority 1 (Core Logic):** Write unit tests with mocked DB for `lib/user.go`, `lib/group.go`, `lib/jwt.go`, and `lib/iptrk.go`.
    *   **Priority 2 (Custom Framework):** Write tests for the reflection-based ORM (`lib/struct.go`) and forms compiler (`ui/form.go`) with various edge cases.
    *   **Priority 3 (UI Handlers):** Use `net/http/httptest` to test HTTP handlers in `ui/root.go` and `ui/form.go` with simulated requests.
4.  **Target Coverage:**
    *   Aim for **>90% coverage** on core business logic (`lib/`) and security-critical components (`lib/jwt.go`, `lib/user_2fa.go`, `lib/iptrk.go`).

---

## Phase 4: CI/CD Pipeline & Code Quality

**Goal:** Automate testing, linting, and coverage reporting to prevent regression.

### Actions:
1.  **GitHub Actions Integration:**
    *   Create `.github/workflows/test.yml` to run on every Pull Request and commit to `main`.
    *   **Job 1 (Linting):** Run `golangci-lint` to enforce style, check for common bugs, and ensure formatting.
    *   **Job 2 (Unit Tests):** Run unit tests (using mocks) without needing a database.
    *   **Job 3 (Integration Tests):** Spin up a Postgres service container, run migrations, and execute database-dependent integration tests.
2.  **Coverage Reporting:**
    *   Integrate a coverage reporting tool (e.g., Codecov or GitHub Actions artifacts) to track coverage over time and block PRs that decrease coverage.

---

## Phase 5: Security Review & Hardening

**Goal:** Conduct a deep, structured security review and establish a vulnerability management process.

### Threat Modeling & Review Areas:
1.  **Custom ORM & SQL Injection:**
    *   *Risk:* Dynamic SQL generation in `lib/struct.go` and query helper methods (`Q_AddWhere`, `Q_AddArg`) in `lib/db.go`.
    *   *Action:* Audit how queries are constructed. Ensure all user input is strictly parameterized and no raw string concatenation of user input occurs.
2.  **Authentication & Session Management:**
    *   *Risk:* JWT token leakage, signature bypass, or weak key usage.
    *   *Action:* Review JWT signing and verification logic in `lib/jwt.go`. Verify token invalidation mechanism (`jwt_invalid` table) works under load. Ensure keys are managed securely and not hardcoded.
3.  **Impersonation (`Become`):**
    *   *Risk:* Unauthorized users triggering the `Become` function to escalate privileges.
    *   *Action:* Thoroughly audit all usages of `ctx.Become`. Enforce that only validated administrators from trusted IPs (SAR) can trigger it.
4.  **Proxy-Safe IP Resolution (XFF):**
    *   *Risk:* IP spoofing leading to SAR bypass or IP lockout evasion.
    *   *Action:* Audit `ParseClientIP` in `lib/misc.go` against various proxy configurations (multiple proxies, spoofed headers).
5.  **Input Validation & XSS:**
    *   *Risk:* XSS in Wiki pages or custom forms.
    *   *Action:* Verify `bluemonday` HTML sanitization is applied consistently to all user-supplied markdown/HTML before rendering. Audit the reflection-based forms validation constraints.
6.  **CSRF Protection:**
    *   *Risk:* CSRF bypass on state-changing POST requests.
    *   *Action:* Verify CSRF token validation in `ui/ui.go` is robust and cannot be bypassed (e.g., by changing content type or omitting the token).

### Security Tooling & Finding Management:
*   **Static Application Security Testing (SAST):** Integrate `gosec` into the linting phase to automatically scan for common Go security issues.
*   **Dependency Scanning:** Integrate `govulncheck` into the CI pipeline to detect known vulnerabilities in dependencies.
*   **Finding Management Plan:**
    *   **Triage:** Classify findings using CVSS (Common Vulnerability Scoring System).
    *   **Remediation SLA:** Establish timelines for fixing vulnerabilities (e.g., Critical: 7 days, High: 30 days, Medium: 90 days).
    *   **Regression Testing:** For every security fix, write a dedicated test case to prevent reintroduction.
