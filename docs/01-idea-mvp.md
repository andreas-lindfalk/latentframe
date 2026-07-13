# PROJECT BLUEPRINT: LATENT FRAME (`latentframe.ai`)

## 1. Executive Summary & Core Value Proposition

Latent Frame is a B2B PropTech and Spatial AI platform built for the premium international real estate market (initial launch: Costa Blanca, Spain). The platform automates the creation of high-end, editorial-style property portals that bridge the cognitive gap for buyers looking at renovation-intent properties.

Instead of generic real estate listings, Latent Frame generates unique standalone portals combining architectural future-state visualizations, photorealistic 3D tracking, and automated financial/geodata analytics.

---

## 2. The Hybrid Spatial Product Architecture

To avoid the high latency, cost, and structural instability of real-time generative 3D environments, Latent Frame utilizes a **2D/3D Hybrid Model**:

1. **The Base Layer (Current State 3D):** A photorealistic 3D environment generated via **3D Gaussian Splatting** or NeRF based on the original property video. The user navigates the current physical state of the building.
2. **The Future Layer (Generative 2D Hotspots):** Strategic high-value interactive hotspots (e.g., kitchen, terrace) embedded in the 3D space. When triggered, they seamlessly overlay high-resolution, stable 2D/360° AI-inpainted images or video sequences demonstrating the property's modern architectural potential.
3. **The Data Layer (Automated Insights):** Automated widgets scraping local land registries and solar APIs to show structural, environmental, and financial ROI parameters.

---

## 3. Technical Pipeline Stack (Phase 1: Deterministic MVP)

The core ingestion engine is a strict, sequential, event-driven pipeline optimized for predictable outputs under 5 minutes. **MCP (Model Context Protocol) is strictly deferred to Phase 2.**

```
[4K Raw Video + Audio] 
       │
       ▼
[Go-Worker (GCP)] ──► FFmpeg (Keyframe extraction based on motion vector analysis)
       │          ──► Google Cloud Storage (Asset hosting via raw GCS URIs)
       ▼
[OpenAI Whisper]  ──► Generates time-stamped text segments of oral instructions
       │
       ▼
[Claude Orchestrator] ─► Matches time-stamps of text + frames. Builds structural prompts.
       │
       ▼
[Flux / ControlNet] ───► Executes targeted 2D Inpainting (Material & lighting surfaces)
       │
       ▼
[Next.js Frontend]  ───► Serves the dynamic hybrid portal to the end-user

```

### Critical API Payload Schema (Go-Worker to Claude Orchestrator)

```json
{
  "project_id": "prop_cabo_roig_001",
  "room_type": "kitchen",
  "global_context": {
    "target_style": "Nordic luxury, minimalist, warm lighting",
    "geo_data": {"country": "Spain", "region": "Alicante", "orientation": "South-West"}
  },
  "extracted_keyframes": [
    {
      "frame_id": "kf_001",
      "timestamp_ms": 1200,
      "gcs_uri": "gs://latentframe-raw-assets/prop_001/kf_001.jpg",
      "camera_angle": "wide_entry"
    }
  ],
  "whisper_transcript": [
    {
      "segment_id": "seg_001",
      "start_ms": 0,
      "end_ms": 3200,
      "text": "Replace old wooden cabinets with matte white handleless facades."
    }
  ]
}

```

---

## 4. Monetization & High-Margin Lead Generation Strategy

The business model deliberately shifts away from manual service fees toward high-velocity digital asset monetization.

* **B2C / Affiliate Layer (The Profit Engine):** Data collected during the scan is instantly monetized through high-intent lead routing. Instead of operational execution, Latent Frame sells warm leads to:
* *Spanish Mortgage Brokers:* Fulfilling non-resident financing requirements (0.5%–1.0% commission on loan volume).
* *Solar & Security Networks:* Direct API lead handoff based on automated PVGIS roof calculations.
* *Turnkey Furniture Providers:* Affiliate packages mapped to the visual output style.


* **B2B Layer (Cash Flow):** Brokerages pay an upfront transactional fee of **€450 per premium property** for the creation of the interactive asset. Data asset ownership remains strictly with Latent Frame.

---

## 5. Immediate Validation & Go-To-Market Protocol (Day 0)

To avoid premature infrastructure investment, Phase 1 utilizes high-touch manual validation with strict operational guardrails:

### Step 1: The Kitchen Isolation Test (Own Property)

* **Action:** Field test the pipeline using the founder's own property in Spain.
* **Guardrail:** Restrict the first test exclusively to **invariant geometry** (transforming colors, textures, lighting, and materials). Do *not* prompt for wall removal or structural layout changes to prevent AI spatial hallucinations. Test the error threshold of the pipeline against camera motion blur and ambient lighting shifts.

### Step 2: The Stale-Listing Pilot Strategy

* **Action:** Execute targeted pilot runs with 2–3 selected premium brokerages on Costa Blanca. Offer the service for zero cost.
* **Guardrail 1 (Perceived Value):** Issues an official invoice of €450 with a applied 100% "Strategic Pilot Discount" to prevent brand devaluation.
* **Guardrail 2 (Traffic Lock):** Sign a binding contract stating the `latentframe.ai` destination link *must* reside on Line 1 of the property description on Idealista and Kyero.
* **Guardrail 3 (Target Criteria):** Only accept pilot properties that have been stagnant on the market for >6 months. Success metrics are judged entirely on new inbound lead velocity for the broker.

---

## 6. Long-Term Scaling Framework (Phase 2)

Once product-market fit is validated via the pilot phase, the human recording bottleneck will be resolved via one of two strategic scale tracks:

* **Track A (Technical):** A custom iOS/Android client application embedding mobile AR engines (ARKit/ARCore). The app forces deterministic user movement via gyroscopic lockouts, real-time blur metrics, and structured audio prompting.
* **Track B (Operational):** A decentralized gig-economy infrastructure ("Uber for Latent Frame"). Vetting and licensing tech-adjacent local freelancers/students using a strict recording protocol, paying €50 per asset ingestion while maintaining the enterprise software margin.
* **Architecture Upgrade:** Integration of distinct Model Context Protocol (MCP) servers (`mcp-server-spain-property` for automated *Catastro* indexing and `mcp-server-interior-index` for automated localized renovation costing) to unlock autonomous agent analytical capabilities.
