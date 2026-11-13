# `promptledger` — Architecture & Engineering Master Plan
> **Git-Native Prompt Version Control, A/B Traffic Router & Shadow Evaluator**  
> *Target: High-Performance Single-Binary Go Engine (< 20MB RAM) + Embedded React 19 Studio*

---

## 1. Executive Summary & Market Problem

### 1.1 The Chaotic Prompt Deployment Reality
In most engineering organizations building AI products:
1. **Unversioned Prompts**: Prompts are either hardcoded strings scattered across backend microservices or stored as unversioned text blobs in a database table.
2. **Zero-Canary Deployments**: When an engineer tweaks a prompt, the change is deployed to 100% of production traffic simultaneously. If the new prompt introduces regressions (e.g. output format violations or subtle hallucinations), all users are immediately impacted.
3. **No Production Shadow Evaluation**: There is no standard, zero-risk way to test a candidate prompt against real production traffic without exposing end-users to potential errors.

### 1.2 The `promptledger` Solution
`promptledger` is an open-source, Git-native prompt versioning daemon and traffic routing proxy written in Go with an embedded React 19 experimentation studio:
- **Git-Native Prompt Registry**: Prompts are version-controlled YAML files (`prompts/classification.v1.yaml`) tracked directly in Git with semantic versioning (SemVer).
- **Dynamic Traffic Splitting**: Routes production requests dynamically (e.g., $90\%$ to `v1.2.0`, $10\%$ to `v1.3.0-canary`) based on user hash, tenant ID, or random split.
- **Zero-Risk Shadow Mode**: Duplicates a fraction of production traffic to a candidate prompt in the background. Measures quality score, latency, and token cost without blocking or affecting the user-facing response.
- **Automated Promotion & Rollback Gates**: Automatically promotes a canary prompt to $100\%$ if it passes quality and latency benchmarks over 24 hours, or triggers an instant rollback if error rates increase.

---

## 2. High-Level System Architecture

```
       +--------------------------------------------------------+
       |             Client / Backend Application               |
       |  Calls: POST /v1/prompts/checkout-intent/execute       |
       +--------------------------------------------------------+
                                   |
                                   v
       +--------------------------------------------------------+
       |                  promptledger Router                   |
       |          (Single Go Binary, Sub-2ms Overhead)          |
       +--------------------------------------------------------+
                                   |
                     +-------------+-------------+
                     |                           | (90% Live Traffic)
                     v                           v
     +-------------------------------+   +-------------------------------+
     |      Prompt v1.2 (Active)     |   |    Prompt v1.3 (Canary 10%)   |
     | - Returns response to user    |   | - Returns response to user    |
     +-------------------------------+   +-------------------------------+
                     |                                   |
                     +-------------+---------------------+
                                   |
                                   | Optional Async Shadow Fork (5%)
                                   v
       +--------------------------------------------------------+
       |               Shadow Evaluation Engine                 |
       |  - Runs Prompt v1.4-experimental in background         |
       |  - Computes Semantic Similarity & Judge Score          |
       |  - Compares P95 Latency & Token Consumption            |
       +--------------------------------------------------------+
                                   |
                                   v
       +--------------------------------------------------------+
       |         SQLite Experimentation Store (WAL Mode)        |
       +--------------------------------------------------------+
                                   |
                                   v
       +--------------------------------------------------------+
       |             React 19 Experimentation Studio            |
       |  - Live A/B Conversion & Accuracy Comparison           |
       |  - Shadow Run Quality Diff Inspector                   |
       |  - 1-Click Rollback / Promote Buttons                  |
       +--------------------------------------------------------+
```

---

## 3. Declarative Prompt Registry Specification (`prompts/intent.yaml`)

```yaml
version: "1"
prompt_id: "checkout-intent-classifier"
description: "Determines user intent during e-commerce checkout flow"

routing:
  strategy: "canary"
  active_version: "v1.2.0"
  variants:
    - version: "v1.2.0"
      weight: 90
    - version: "v1.3.0-canary"
      weight: 10
  shadow:
    enabled: true
    version: "v1.4.0-shadow"
    sample_rate: 0.05 # 5% of traffic

template: |
  You are an expert e-commerce assistant.
  Classify the user intent into: [PURCHASE, INQUIRY, COMPLAINT, REFUND].
  User Input: {{.UserInput}}
```

---

## 4. CLI Command Surface

```bash
# Start promptledger router & web studio
promptledger serve --config ./promptledger.yaml --port 8080

# Validate prompt templates and variable bindings
promptledger validate ./prompts/

# Promote a canary prompt to 100% active traffic
promptledger promote --prompt checkout-intent-classifier --version v1.3.0-canary

# Trigger an immediate rollback to previous version
promptledger rollback --prompt checkout-intent-classifier
```

---

## 5. Implementation Roadmap (5 Phases)

### Phase 1: Git-Native Registry & Template Engine
- Build YAML prompt parser with Mustache/Go-template variable substitution.
- Implement Git watcher detecting prompt file changes automatically.

### Phase 2: Traffic Splitting & Canary Router
- Implement deterministic weighted traffic router based on user/tenant hashing.
- Maintain sub-2ms proxy routing overhead.

### Phase 3: Async Shadow Mode & Evaluator
- Implement non-blocking background goroutine executor for shadow prompts.
- Compute automated LLM-as-a-judge comparison scores against production answers.

### Phase 4: Automated Safety Gates & Instant Rollback
- Implement statistical test comparing error rates and latency between variants.
- Automated rollback trigger if canary error rate exceeds baseline by $> 1\%$.

### Phase 5: Embedded React 19 Studio & Documentation
- Build live A/B visual dashboard with shadow diff viewers.
- Package as a single Go binary under 20MB RAM.
