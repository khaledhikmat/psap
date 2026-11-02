import { useEffect, useMemo, useState, type FormEvent } from "react";
import { get, post } from "./api";
import type {
  Asset,
  AssetRequest,
  Baseline,
  Launch,
  PredefinedAnswer,
  Question,
  QuestionCategory,
  Tier,
  UserAnswerDetail,
  Worker,
} from "./types";

type View = "dashboard" | "asset" | "launch";
type Notice = { kind: "success" | "error" | "info"; text: string };

const emptyArray = <T,>(): T[] => [];
const waitingForAnswers = "waiting-4-answers";

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : "Something went wrong.";
}

function assetStateLabel(state: string): string {
  const labels: Record<string, string> = {
    "pending-launch": "Needs LAUNCH",
    launched: "Launch complete",
    "pending-tier-validation": "Awaiting validation",
    "pending-shield": "Shield assessment",
    shielded: "Shield complete",
    "pending-security-assessment": "Security assessment",
    "pending-security-implementation": "Implementation",
    implemented: "Implemented",
  };
  return labels[state] ?? state.replaceAll("-", " ");
}

function launchStateLabel(state: string): string {
  const labels: Record<string, string> = {
    "waiting-4-answers": "Owner input needed",
    "answers-submitted": "Submitted for review",
    "waiting-4-tier-approval": "Awaiting review",
    "tier-validated": "Validated",
    cancelled: "Replaced",
    timedout: "Timed out",
  };
  return labels[state] ?? state.replaceAll("-", " ");
}

function locationLabel(asset: Asset): string {
  return [
    asset.campus,
    asset.building,
    asset.floor ? `Floor ${asset.floor}` : "",
    asset.space,
  ]
    .filter(Boolean)
    .join(" · ");
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat(undefined, {
        month: "short",
        day: "numeric",
        year: "numeric",
      }).format(date);
}

function App() {
  const [assets, setAssets] = useState<Asset[]>([]);
  const [launches, setLaunches] = useState<Launch[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);
  const [tiers, setTiers] = useState<Tier[]>([]);
  const [ownerId, setOwnerId] = useState("");
  const [view, setView] = useState<View>("dashboard");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [loadError, setLoadError] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [showRequestDialog, setShowRequestDialog] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState<Asset | null>(null);
  const [assetLaunches, setAssetLaunches] = useState<Launch[]>([]);
  const [assetBaselines, setAssetBaselines] = useState<Baseline[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [activeLaunch, setActiveLaunch] = useState<Launch | null>(null);
  const [categories, setCategories] = useState<QuestionCategory[]>([]);
  const [questions, setQuestions] = useState<Question[]>([]);
  const [predefinedAnswers, setPredefinedAnswers] = useState<
    PredefinedAnswer[]
  >([]);
  const [launchAnswers, setLaunchAnswers] = useState<
    Record<string, string[]>
  >({});
  const [savedAnswers, setSavedAnswers] = useState<Record<string, string[]>>(
    {},
  );
  const [categoryIndex, setCategoryIndex] = useState(0);
  const [launchLoading, setLaunchLoading] = useState(false);
  const [launchSaving, setLaunchSaving] = useState(false);
  const [launchError, setLaunchError] = useState("");

  async function refreshDashboard() {
    setIsLoading(true);
    setLoadError("");
    try {
      const [assetRows, launchRows, workerRows, tierRows] = await Promise.all([
        get<Asset[]>("/assets"),
        get<Launch[]>("/launches"),
        get<Worker[]>("/workers"),
        get<Tier[]>("/tiers"),
      ]);
      setAssets(assetRows ?? emptyArray());
      setLaunches(launchRows ?? emptyArray());
      setWorkers(workerRows ?? emptyArray());
      setTiers(tierRows ?? emptyArray());
    } catch (error) {
      setLoadError(errorText(error));
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void refreshDashboard();
  }, []);

  const ownerOptions = useMemo(() => {
    const assignedOwnerIds = new Set(
      assets.map((asset) => asset.businessOwner).filter(Boolean),
    );
    const assignedOwners = workers.filter((worker) =>
      assignedOwnerIds.has(worker.id),
    );
    return assignedOwners.length > 0 ? assignedOwners : workers;
  }, [assets, workers]);

  useEffect(() => {
    if (
      ownerOptions.length > 0 &&
      !ownerOptions.some((worker) => worker.id === ownerId)
    ) {
      setOwnerId(ownerOptions[0].id);
    }
  }, [ownerId, ownerOptions]);

  const ownerAssets = useMemo(
    () => assets.filter((asset) => asset.businessOwner === ownerId),
    [assets, ownerId],
  );
  const ownerAssetIds = useMemo(
    () => new Set(ownerAssets.map((asset) => asset.id)),
    [ownerAssets],
  );
  const ownerLaunches = useMemo(
    () => launches.filter((launch) => ownerAssetIds.has(launch.assetId)),
    [launches, ownerAssetIds],
  );
  const pendingLaunches = useMemo(
    () =>
      ownerLaunches
        .filter((launch) => launch.state === waitingForAnswers)
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt)),
    [ownerLaunches],
  );
  const managerOptions = useMemo(() => {
    const managerIds = new Set(
      assets.map((asset) => asset.businessManager).filter(Boolean),
    );
    const managers = workers.filter((worker) => managerIds.has(worker.id));
    return managers.length > 0
      ? managers
      : workers.filter((worker) => worker.id !== ownerId);
  }, [assets, ownerId, workers]);
  const owner = ownerOptions.find((worker) => worker.id === ownerId);
  const tierById = useMemo(
    () => new Map(tiers.map((tier) => [tier.id, tier])),
    [tiers],
  );

  async function loadAsset(asset: Asset) {
    setSelectedAsset(asset);
    setView("asset");
    setDetailLoading(true);
    setNotice(null);
    try {
      const [history, baselines] = await Promise.all([
        get<Launch[]>(`/assets/${encodeURIComponent(asset.id)}/launches`),
        get<Baseline[]>(`/assets/${encodeURIComponent(asset.id)}/baselines`),
      ]);
      setAssetLaunches(
        (history ?? []).sort((a, b) =>
          b.createdAt.localeCompare(a.createdAt),
        ),
      );
      setAssetBaselines(baselines ?? []);
    } catch (error) {
      setNotice({ kind: "error", text: errorText(error) });
      setAssetLaunches([]);
      setAssetBaselines([]);
    } finally {
      setDetailLoading(false);
    }
  }

  async function loadLaunch(launch: Launch) {
    setActiveLaunch(null);
    setLaunchLoading(true);
    setLaunchError("");
    setNotice(null);
    try {
      const [categoryRows, questionRows, answerRows, savedRows] =
        await Promise.all([
          get<QuestionCategory[]>("/question-categories?type=launch"),
          get<Question[]>("/questions?categoryType=launch"),
          get<PredefinedAnswer[]>("/predefined-answers?type=launch"),
          get<UserAnswerDetail[]>(
            `/launches/${encodeURIComponent(launch.id)}/answers`,
          ),
        ]);
      const orderedCategories = (categoryRows ?? []).sort((a, b) =>
        a.createdAt.localeCompare(b.createdAt),
      );
      const orderedQuestions = (questionRows ?? []).sort(
        (a, b) => a.displayOrder - b.displayOrder || a.id.localeCompare(b.id),
      );
      const orderedAnswers = (answerRows ?? []).sort(
        (a, b) => a.displayOrder - b.displayOrder || a.id.localeCompare(b.id),
      );
      const initial: Record<string, string[]> = {};
      for (const answer of savedRows ?? []) {
        initial[answer.questionId] = answer.selections.map(
          (selection) => selection.predefinedAnswerId,
        );
      }
      setActiveLaunch(launch);
      setCategories(orderedCategories);
      setQuestions(orderedQuestions);
      setPredefinedAnswers(orderedAnswers);
      setLaunchAnswers(initial);
      setSavedAnswers(initial);
      setCategoryIndex(0);
      setView("launch");
    } catch (error) {
      setLaunchError(errorText(error));
      setView("launch");
    } finally {
      setLaunchLoading(false);
    }
  }

  async function beginLaunch(asset: Asset, isNewRequest = false) {
    setNotice({
      kind: "info",
      text: isNewRequest
        ? "Asset request saved. Starting its LAUNCH questionnaire…"
        : "Starting a new LAUNCH questionnaire…",
    });
    const previousLaunchId = asset.lastLaunchId ?? "";
    try {
      await post("/launches", { assetId: asset.id });
      const deadline = Date.now() + 8000;
      while (Date.now() < deadline) {
        await new Promise((resolve) => window.setTimeout(resolve, 400));
        const [freshAsset, history] = await Promise.all([
          get<Asset>(`/assets/${encodeURIComponent(asset.id)}`),
          get<Launch[]>(
            `/assets/${encodeURIComponent(asset.id)}/launches`,
          ),
        ]);
        if (
          freshAsset.lastLaunchId &&
          freshAsset.lastLaunchId !== previousLaunchId
        ) {
          const createdLaunch = (history ?? []).find(
            (launch) => launch.id === freshAsset.lastLaunchId,
          );
          if (createdLaunch) {
            await refreshDashboard();
            await loadLaunch(createdLaunch);
            setNotice({
              kind: "success",
              text: "LAUNCH is ready. Your answers are saved only when you choose Save progress.",
            });
            return;
          }
        }
      }
      await refreshDashboard();
      setNotice({
        kind: "info",
        text: "The LAUNCH request was accepted. It will appear here when the workflow worker creates it.",
      });
    } catch (error) {
      await refreshDashboard();
      setNotice({
        kind: "error",
        text: isNewRequest
          ? `The asset request was saved, but LAUNCH could not be started: ${errorText(error)}`
          : `LAUNCH could not be started: ${errorText(error)}`,
      });
    }
  }

  async function createAssetRequest(request: AssetRequest) {
    const created = await post<Asset>("/assets", request);
    setShowRequestDialog(false);
    await refreshDashboard();
    await beginLaunch(created, true);
  }

  function changeAnswer(question: Question, answerId: string) {
    setLaunchError("");
    setLaunchAnswers((current) => {
      const selected = current[question.id] ?? [];
      if (!question.allowMultipleAnswers) {
        return { ...current, [question.id]: [answerId] };
      }
      if (selected.includes(answerId)) {
        const next = selected.filter((id) => id !== answerId);
        if (
          next.length === 0 &&
          (savedAnswers[question.id]?.length ?? 0) > 0
        ) {
          setLaunchError(
            "Keep at least one choice selected for a saved question. The API does not support clearing an answer.",
          );
          return current;
        }
        return { ...current, [question.id]: next };
      }
      return { ...current, [question.id]: [...selected, answerId] };
    });
  }

  async function saveLaunch(finalize = false) {
    if (!activeLaunch) return;
    const unanswered = questions.filter(
      (question) => (launchAnswers[question.id] ?? []).length === 0,
    ).length;
    if (
      finalize &&
      unanswered > 0 &&
      !window.confirm(
        `${unanswered} question${unanswered === 1 ? "" : "s"} will remain unanswered. You can still submit this LAUNCH for review. Continue?`,
      )
    ) {
      return;
    }

    const submissions = questions
      .filter((question) => (launchAnswers[question.id] ?? []).length > 0)
      .map((question) => ({
        questionId: question.id,
        predefinedAnswerIds: launchAnswers[question.id],
      }));
    setLaunchSaving(true);
    setLaunchError("");
    try {
      await post(
        `/launches/${encodeURIComponent(activeLaunch.id)}/${
          finalize ? "finalize-answers" : "submit-answers"
        }`,
        { answers: submissions },
      );
      if (finalize) {
        setNotice({
          kind: "success",
          text: "LAUNCH submitted. Your business manager or security assessor will review it.",
        });
        await refreshDashboard();
        setView("dashboard");
        setActiveLaunch(null);
      } else {
        setSavedAnswers(launchAnswers);
        setNotice({ kind: "success", text: "Your progress has been saved." });
      }
    } catch (error) {
      setLaunchError(errorText(error));
    } finally {
      setLaunchSaving(false);
    }
  }

  function goToDashboard() {
    setView("dashboard");
    setSelectedAsset(null);
    setActiveLaunch(null);
    setNotice(null);
  }

  const category = categories[categoryIndex];
  const categoryQuestions = category
    ? questions.filter((question) => question.categoryId === category.id)
    : [];
  const unansweredCount = questions.filter(
    (question) => (launchAnswers[question.id] ?? []).length === 0,
  ).length;

  useEffect(() => {
    if (view !== "launch" || categories.length < 2) return;
    function onKeyDown(event: KeyboardEvent) {
      if (!event.altKey || !["ArrowLeft", "ArrowRight"].includes(event.key)) {
        return;
      }
      event.preventDefault();
      setCategoryIndex((current) =>
        event.key === "ArrowRight"
          ? Math.min(current + 1, categories.length - 1)
          : Math.max(current - 1, 0),
      );
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [categories.length, view]);

  return (
    <div className="app-shell">
      <header className="topbar">
        <button className="brand" onClick={goToDashboard} aria-label="PSAP home">
          <span className="brand-mark">P</span>
          <span className="brand-name">
            PSAP <small>Physical Security Assessment Program</small>
          </span>
        </button>
        <div className="topbar-right">
          <span className="demo-label">
            <span className="demo-dot" /> DEMO
          </span>
          <label className="owner-picker">
            <span>Viewing as</span>
            <select
              aria-label="Select demo business owner"
              value={ownerId}
              onChange={(event) => setOwnerId(event.target.value)}
              disabled={ownerOptions.length === 0}
            >
              {ownerOptions.map((worker) => (
                <option key={worker.id} value={worker.id}>
                  {worker.name}
                </option>
              ))}
            </select>
          </label>
          <div className="avatar" aria-hidden="true">
            {owner?.name
              ?.split(/\s+/)
              .map((part) => part[0])
              .join("")
              .slice(0, 2) ?? "BO"}
          </div>
        </div>
      </header>

      <div className="demo-banner">
        Demo environment · No authentication · Changes are stored by the PSAP API
      </div>

      <main className="main-content">
        {notice && (
          <div className={`notice notice-${notice.kind}`} role="status">
            <span>{notice.text}</span>
            <button
              className="notice-close"
              onClick={() => setNotice(null)}
              aria-label="Dismiss message"
            >
              ×
            </button>
          </div>
        )}

        {loadError && (
          <div className="error-state" role="alert">
            <div>
              <strong>Could not load PSAP data</strong>
              <p>{loadError}</p>
              <p className="error-hint">
                Make sure the backend API is running on localhost:8080.
              </p>
            </div>
            <button className="button button-secondary" onClick={refreshDashboard}>
              Try again
            </button>
          </div>
        )}

        {isLoading && !loadError && (
          <div className="loading-state">
            <span className="spinner" /> Connecting to PSAP…
          </div>
        )}

        {!isLoading && !loadError && view === "dashboard" && (
          <section aria-labelledby="dashboard-title">
            <div className="page-heading heading-row">
              <div>
                <div className="eyebrow">BUSINESS OWNER PORTAL</div>
                <h1 id="dashboard-title">Good to see you, {owner?.name ?? "Owner"}</h1>
                <p>
                  Manage your secure spaces and keep their business assessments
                  up to date.
                </p>
              </div>
              <button
                className="button button-primary"
                onClick={() => setShowRequestDialog(true)}
                disabled={workers.length === 0}
              >
                <span className="button-plus">＋</span> Request a new asset
              </button>
            </div>

            <div className="metric-grid">
              <Metric
                label="Assets you own"
                value={ownerAssets.length}
                caption="Secure rooms and areas"
                accent="blue"
              />
              <Metric
                label="LAUNCH actions"
                value={pendingLaunches.length}
                caption="Waiting for your input"
                accent={pendingLaunches.length > 0 ? "amber" : "green"}
              />
              <Metric
                label="Tiered assets"
                value={ownerAssets.filter((asset) => asset.tierId).length}
                caption="Validated by security"
                accent="navy"
              />
            </div>

            <div className="dashboard-grid">
              <section className="panel work-panel" aria-labelledby="work-title">
                <div className="panel-heading">
                  <div>
                    <div className="eyebrow">YOUR NEXT STEP</div>
                    <h2 id="work-title">LAUNCH tasks</h2>
                  </div>
                  <span className="count-badge">{pendingLaunches.length}</span>
                </div>
                {pendingLaunches.length === 0 ? (
                  <div className="empty-state compact-empty">
                    <span className="empty-icon">✓</span>
                    <strong>You’re all caught up</strong>
                    <span>No LAUNCH questionnaires need your input.</span>
                  </div>
                ) : (
                  <div className="task-list">
                    {pendingLaunches.map((launch) => {
                      const asset = assets.find(
                        (row) => row.id === launch.assetId,
                      );
                      return (
                        <article className="task-card" key={launch.id}>
                          <div className="task-icon" aria-hidden="true">
                            L
                          </div>
                          <div className="task-copy">
                            <strong>{asset?.name ?? launch.assetId}</strong>
                            <span>
                              {asset ? locationLabel(asset) : "Asset details loading"}
                            </span>
                            <small>
                              Started {formatDate(launch.createdAt)} ·{" "}
                              {launchStateLabel(launch.state)}
                            </small>
                          </div>
                          <button
                            className="button button-small button-primary"
                            onClick={() => void loadLaunch(launch)}
                          >
                            Continue <span aria-hidden="true">→</span>
                          </button>
                        </article>
                      );
                    })}
                  </div>
                )}
              </section>

              <aside className="panel principle-panel">
                <div className="principle-symbol" aria-hidden="true">
                  i
                </div>
                <div className="eyebrow">A QUICK REMINDER</div>
                <h2>LAUNCH is about business impact</h2>
                <p>
                  Describe what the room supports and what would happen if it
                  were unavailable. Security will validate your answers and
                  confirm the tier.
                </p>
                <div className="principle-rule">
                  You won’t be asked to choose cameras, alarms, or other
                  security controls.
                </div>
              </aside>
            </div>

            <section className="assets-section" aria-labelledby="assets-title">
              <div className="section-heading">
                <div>
                  <div className="eyebrow">YOUR PORTFOLIO</div>
                  <h2 id="assets-title">Assets you own</h2>
                </div>
                <span className="muted">{ownerAssets.length} total</span>
              </div>
              {ownerAssets.length === 0 ? (
                <div className="panel empty-assets">
                  <div className="empty-icon">⌂</div>
                  <h3>No assets assigned yet</h3>
                  <p>
                    Request a new secure room or area to begin its LAUNCH
                    assessment.
                  </p>
                  <button
                    className="button button-primary"
                    onClick={() => setShowRequestDialog(true)}
                  >
                    Request an asset
                  </button>
                </div>
              ) : (
                <div className="asset-grid">
                  {ownerAssets.map((asset) => {
                    const active = ownerLaunches.find(
                      (launch) =>
                        launch.assetId === asset.id &&
                        launch.state === waitingForAnswers,
                    );
                    const tier = asset.tierId
                      ? tierById.get(asset.tierId)
                      : undefined;
                    return (
                      <article className="asset-card" key={asset.id}>
                        <button
                          className="asset-card-main"
                          onClick={() => void loadAsset(asset)}
                          aria-label={`View ${asset.name} details`}
                        >
                          <div className="asset-card-top">
                            <span className="asset-type-icon" aria-hidden="true">
                              <span />
                            </span>
                            <span
                              className={`state-pill state-${asset.state}`}
                            >
                              {assetStateLabel(asset.state)}
                            </span>
                          </div>
                          <h3>{asset.name}</h3>
                          <p className="asset-location">{locationLabel(asset)}</p>
                          <div className="asset-card-bottom">
                            <span>
                              {tier ? (
                                <>
                                  <span className="tier-dot" />
                                  {tier.name} · {tier.title}
                                </>
                              ) : (
                                <span className="muted">Tier not assigned</span>
                              )}
                            </span>
                            <span className="asset-open" aria-hidden="true">
                              →
                            </span>
                          </div>
                        </button>
                        {active ? (
                          <button
                            className="asset-action"
                            onClick={() => void loadLaunch(active)}
                          >
                            Continue LAUNCH <span aria-hidden="true">→</span>
                          </button>
                        ) : (
                          <button
                            className="asset-action"
                            onClick={() => void beginLaunch(asset)}
                          >
                            {!asset.lastLaunchId &&
                            asset.state === "pending-launch"
                              ? "Start LAUNCH"
                              : "Request reassessment"}{" "}
                            <span aria-hidden="true">→</span>
                          </button>
                        )}
                      </article>
                    );
                  })}
                </div>
              )}
            </section>

            <footer className="page-footer">
              <span>PSAP · Secure room and area assessment</span>
              <button className="text-button" onClick={refreshDashboard}>
                Refresh data
              </button>
            </footer>
          </section>
        )}

        {!isLoading && !loadError && view === "asset" && selectedAsset && (
          <section aria-labelledby="asset-title">
            <button className="back-link" onClick={goToDashboard}>
              <span aria-hidden="true">←</span> Back to my assets
            </button>
            <div className="page-heading detail-heading">
              <div>
                <div className="eyebrow">ASSET DETAILS</div>
                <h1 id="asset-title">{selectedAsset.name}</h1>
                <p>{locationLabel(selectedAsset)}</p>
              </div>
              <div className="detail-actions">
                {assetLaunches.find(
                  (launch) => launch.state === waitingForAnswers,
                ) ? (
                  <button
                    className="button button-primary"
                    onClick={() =>
                      void loadLaunch(
                        assetLaunches.find(
                          (launch) => launch.state === waitingForAnswers,
                        )!,
                      )
                    }
                  >
                    Continue LAUNCH
                  </button>
                ) : (
                  <button
                    className="button button-primary"
                    onClick={() => void beginLaunch(selectedAsset)}
                  >
                    {assetLaunches.length === 0 &&
                    !selectedAsset.lastLaunchId &&
                    selectedAsset.state === "pending-launch"
                      ? "Start LAUNCH"
                      : "Request reassessment"}
                  </button>
                )}
              </div>
            </div>

            {detailLoading ? (
              <div className="loading-state">
                <span className="spinner" /> Loading asset history…
              </div>
            ) : (
              <>
                <div className="detail-summary-grid">
                  <div className="panel detail-summary">
                    <div className="eyebrow">CURRENT STATUS</div>
                    <span
                      className={`state-pill state-${selectedAsset.state}`}
                    >
                      {assetStateLabel(selectedAsset.state)}
                    </span>
                    <h2>
                      {selectedAsset.tierId
                        ? tierById.get(selectedAsset.tierId)?.name ??
                          "Tier assigned"
                        : "Tier not assigned"}
                    </h2>
                    <p>
                      {selectedAsset.tierId
                        ? tierById.get(selectedAsset.tierId)?.title ??
                          "Confirmed by the assessment process."
                        : "A tier is confirmed after a business assessment is reviewed."}
                    </p>
                  </div>
                  <div className="panel location-panel">
                    <div className="eyebrow">LOCATION</div>
                    <dl className="location-list">
                      <div>
                        <dt>Campus</dt>
                        <dd>{selectedAsset.campus || "—"}</dd>
                      </div>
                      <div>
                        <dt>Building</dt>
                        <dd>{selectedAsset.building || "—"}</dd>
                      </div>
                      <div>
                        <dt>Floor / space</dt>
                        <dd>
                          {[selectedAsset.floor, selectedAsset.space]
                            .filter(Boolean)
                            .join(" · ") || "—"}
                        </dd>
                      </div>
                    </dl>
                  </div>
                </div>

                <div className="panel history-panel">
                  <div className="panel-heading">
                    <div>
                      <div className="eyebrow">ASSESSMENT RECORD</div>
                      <h2>LAUNCH history</h2>
                    </div>
                    <span className="muted">
                      {assetLaunches.length}{" "}
                      {assetLaunches.length === 1 ? "assessment" : "assessments"}
                    </span>
                  </div>
                  {assetLaunches.length === 0 ? (
                    <p className="muted">
                      No LAUNCH assessments have been started for this asset.
                    </p>
                  ) : (
                    <div className="history-list">
                      {assetLaunches.map((launch) => (
                        <div className="history-row" key={launch.id}>
                          <div className="history-marker" aria-hidden="true" />
                          <div className="history-copy">
                            <strong>{launchStateLabel(launch.state)}</strong>
                            <span>
                              Started {formatDate(launch.createdAt)}
                            </span>
                          </div>
                          <span className="muted">
                            {launch.tierId
                              ? tierById.get(launch.tierId)?.name ??
                                "Tier review"
                              : "Tier pending"}
                          </span>
                          {launch.state === waitingForAnswers && (
                            <button
                              className="text-button"
                              onClick={() => void loadLaunch(launch)}
                            >
                              Open LAUNCH
                            </button>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div className="detail-lower-grid">
                  <div className="panel">
                    <div className="eyebrow">RELATED RECORDS</div>
                    <h2>Baselines</h2>
                    {assetBaselines.length === 0 ? (
                      <p className="muted">
                        Baselines will appear after the assessment and security
                        review are complete.
                      </p>
                    ) : (
                      <ul className="record-list">
                        {assetBaselines.map((baseline) => (
                          <li key={baseline.id}>
                            <span>{baseline.id}</span>
                            <span className="state-pill">
                              {baseline.state.replaceAll("-", " ")}
                            </span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                  <div className="panel guide-panel">
                    <div className="eyebrow">BUSINESS OWNER GUIDE</div>
                    <h2>What to expect</h2>
                    <ul className="guide-list">
                      <li>Answer based on the room as it is used today.</li>
                      <li>Describe business impact, not security equipment.</li>
                      <li>
                        Security reviews the answers and confirms the tier.
                      </li>
                      <li>
                        A tier changes only through a formal reassessment.
                      </li>
                    </ul>
                    <p className="guide-note">
                      SHIELD measures observable exposure; it does not change
                      this asset’s tier.
                    </p>
                  </div>
                </div>
              </>
            )}
          </section>
        )}

        {view === "launch" && (
          <section className="launch-view" aria-labelledby="launch-title">
            <button className="back-link" onClick={goToDashboard}>
              <span aria-hidden="true">←</span> Back to my overview
            </button>
            {launchLoading ? (
              <div className="loading-state">
                <span className="spinner" /> Loading your LAUNCH…
              </div>
            ) : launchError && !activeLaunch ? (
              <div className="error-state" role="alert">
                <div>
                  <strong>Could not open this LAUNCH</strong>
                  <p>{launchError}</p>
                </div>
                <button className="button button-secondary" onClick={goToDashboard}>
                  Return to overview
                </button>
              </div>
            ) : activeLaunch ? (
              <>
                <div className="page-heading launch-heading">
                  <div>
                    <div className="eyebrow">BUSINESS IMPACT ASSESSMENT</div>
                    <h1 id="launch-title">
                      {assets.find((asset) => asset.id === activeLaunch.assetId)
                        ?.name ?? "LAUNCH questionnaire"}
                    </h1>
                    <p>
                      Answer for the actual room or area. Your business manager
                      or security assessor will validate the result.
                    </p>
                  </div>
                  <span className="draft-indicator">
                    <span className="draft-dot" />
                    {Object.keys(savedAnswers).length > 0
                      ? "Saved progress"
                      : "Not yet saved"}
                  </span>
                </div>

                <div className="launch-layout">
                  <aside className="step-panel" aria-label="Question categories">
                    <div className="step-panel-title">LAUNCH CATEGORIES</div>
                    <div className="step-list">
                      {categories.map((item, index) => {
                        const itemQuestions = questions.filter(
                          (question) => question.categoryId === item.id,
                        );
                        const complete =
                          itemQuestions.length > 0 &&
                          itemQuestions.every(
                            (question) =>
                              (launchAnswers[question.id] ?? []).length > 0,
                          );
                        return (
                          <button
                            className={`step-button ${
                              index === categoryIndex ? "step-current" : ""
                            } ${complete ? "step-complete" : ""}`}
                            key={item.id}
                            aria-current={
                              index === categoryIndex ? "step" : undefined
                            }
                            onClick={() => setCategoryIndex(index)}
                          >
                            <span className="step-number">
                              {complete ? "✓" : String(index + 1).padStart(2, "0")}
                            </span>
                            <span className="step-label">{item.title}</span>
                            <span className="step-status">
                              {complete ? "Done" : "Open"}
                            </span>
                          </button>
                        );
                      })}
                    </div>
                    <div className="step-help">
                      <strong>Keyboard tip</strong>
                      <span>
                        Use Alt + ← / → to move between categories.
                      </span>
                    </div>
                  </aside>

                  <div className="question-panel">
                    {category ? (
                      <>
                        <div className="question-topline">
                          <span className="eyebrow">
                            CATEGORY {String(categoryIndex + 1).padStart(2, "0")}{" "}
                            OF {String(categories.length).padStart(2, "0")}
                          </span>
                          <span className="optional-label">
                            Business owner input
                          </span>
                        </div>
                        <h2>{category.title}</h2>
                        {category.text && (
                          <p className="category-intro">{category.text}</p>
                        )}
                        {categoryQuestions.length === 0 ? (
                          <div className="inline-error" role="alert">
                            No questions are configured for this category.
                          </div>
                        ) : (
                          categoryQuestions.map((question) => {
                            const selected =
                              launchAnswers[question.id] ?? [];
                            const options = predefinedAnswers.filter(
                              (answer) => answer.questionId === question.id,
                            );
                            return (
                              <fieldset
                                className="question-fieldset"
                                key={question.id}
                              >
                                <legend>{question.text}</legend>
                                {question.allowMultipleAnswers && (
                                  <p className="question-hint">
                                    Select all that apply.
                                  </p>
                                )}
                                <div className="answer-options">
                                  {options.map((answer, answerIndex) => {
                                    const checked = selected.includes(answer.id);
                                    const inputId = `${question.id}-${answer.id}`;
                                    return (
                                      <label
                                        className={`answer-option ${
                                          checked ? "answer-selected" : ""
                                        }`}
                                        htmlFor={inputId}
                                        key={answer.id}
                                      >
                                        <input
                                          id={inputId}
                                          type={
                                            question.allowMultipleAnswers
                                              ? "checkbox"
                                              : "radio"
                                          }
                                          name={question.id}
                                          value={answer.id}
                                          checked={checked}
                                          onChange={() =>
                                            changeAnswer(question, answer.id)
                                          }
                                        />
                                        <span className="answer-copy">
                                          <span className="answer-text">
                                            {answer.text}
                                          </span>
                                          {answer.points > 0 && (
                                            <span className="answer-order">
                                              {String(answerIndex + 1).padStart(
                                                2,
                                                "0",
                                              )}
                                            </span>
                                          )}
                                        </span>
                                      </label>
                                    );
                                  })}
                                </div>
                              </fieldset>
                            );
                          })
                        )}
                        <div className="question-footer">
                          <button
                            className="button button-secondary"
                            onClick={() =>
                              setCategoryIndex((index) =>
                                Math.max(0, index - 1),
                              )
                            }
                            disabled={categoryIndex === 0}
                          >
                            ← Previous
                          </button>
                          <span className="muted">
                            {unansweredCount}{" "}
                            {unansweredCount === 1 ? "question" : "questions"}{" "}
                            unanswered
                          </span>
                          <button
                            className="button button-secondary"
                            onClick={() =>
                              setCategoryIndex((index) =>
                                Math.min(categories.length - 1, index + 1),
                              )
                            }
                            disabled={categoryIndex === categories.length - 1}
                          >
                            Next category →
                          </button>
                        </div>
                      </>
                    ) : (
                      <div className="empty-state">
                        <strong>No LAUNCH categories are available.</strong>
                        <span>Check the API question configuration.</span>
                      </div>
                    )}
                    {launchError && (
                      <div className="inline-error" role="alert">
                        {launchError}
                      </div>
                    )}
                    <div className="launch-actions">
                      <button
                        className="button button-secondary"
                        onClick={() => void saveLaunch(false)}
                        disabled={launchSaving || questions.length === 0}
                      >
                        {launchSaving ? "Saving…" : "Save progress"}
                      </button>
                      <button
                        className="button button-primary"
                        onClick={() => void saveLaunch(true)}
                        disabled={launchSaving || questions.length === 0}
                      >
                        {launchSaving ? "Submitting…" : "Submit LAUNCH"}
                      </button>
                    </div>
                    <p className="privacy-note">
                      Your answers are stored by the PSAP API. You may save
                      partial progress and return later.
                    </p>
                  </div>
                </div>
              </>
            ) : (
              <div className="empty-state">
                <strong>No LAUNCH selected.</strong>
                <button className="button button-secondary" onClick={goToDashboard}>
                  Return to overview
                </button>
              </div>
            )}
          </section>
        )}
      </main>

      {showRequestDialog && (
        <AssetRequestDialog
          ownerId={ownerId}
          ownerName={owner?.name ?? "Business owner"}
          owners={ownerOptions}
          managers={managerOptions}
          onClose={() => setShowRequestDialog(false)}
          onSubmit={createAssetRequest}
        />
      )}
    </div>
  );
}

function Metric({
  label,
  value,
  caption,
  accent,
}: {
  label: string;
  value: number;
  caption: string;
  accent: "blue" | "amber" | "green" | "navy";
}) {
  return (
    <div className={`metric-card metric-${accent}`}>
      <div className="metric-label">{label}</div>
      <div className="metric-value">{value}</div>
      <div className="metric-caption">{caption}</div>
    </div>
  );
}

function AssetRequestDialog({
  ownerId,
  ownerName,
  owners,
  managers,
  onClose,
  onSubmit,
}: {
  ownerId: string;
  ownerName: string;
  owners: Worker[];
  managers: Worker[];
  onClose: () => void;
  onSubmit: (request: AssetRequest) => Promise<void>;
}) {
  const [form, setForm] = useState<AssetRequest>({
    name: "",
    campus: "",
    building: "",
    floor: "",
    space: "",
    priority: "normal",
    businessOwnerId: ownerId,
    businessManagerId: managers[0]?.id ?? "",
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    setForm((current) => ({
      ...current,
      businessOwnerId: ownerId || current.businessOwnerId,
      businessManagerId:
        current.businessManagerId &&
        managers.some((manager) => manager.id === current.businessManagerId)
          ? current.businessManagerId
          : (managers[0]?.id ?? ""),
    }));
  }, [ownerId, managers]);

  function update(field: keyof AssetRequest, value: string) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await onSubmit(form);
    } catch (submitError) {
      setError(errorText(submitError));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && !submitting) onClose();
      }}
    >
      <section
        className="request-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="request-title"
      >
        <div className="dialog-heading">
          <div>
            <div className="eyebrow">NEW SECURE SPACE</div>
            <h2 id="request-title">Request an asset</h2>
            <p>
              Add the room or area details. A LAUNCH questionnaire will be
              created next.
            </p>
          </div>
          <button
            className="dialog-close"
            onClick={onClose}
            disabled={submitting}
            aria-label="Close dialog"
          >
            ×
          </button>
        </div>
        <form onSubmit={submit}>
          <div className="form-grid">
            <label className="form-field form-full">
              <span>Room or area name *</span>
              <input
                required
                autoFocus
                value={form.name}
                onChange={(event) => update("name", event.target.value)}
                placeholder="e.g. North Campus Archive"
              />
            </label>
            <label className="form-field">
              <span>Campus *</span>
              <input
                required
                value={form.campus}
                onChange={(event) => update("campus", event.target.value)}
                placeholder="e.g. North Campus"
              />
            </label>
            <label className="form-field">
              <span>Building *</span>
              <input
                required
                value={form.building}
                onChange={(event) => update("building", event.target.value)}
                placeholder="e.g. A2"
              />
            </label>
            <label className="form-field">
              <span>Floor</span>
              <input
                value={form.floor}
                onChange={(event) => update("floor", event.target.value)}
                placeholder="e.g. 2"
              />
            </label>
            <label className="form-field">
              <span>Room / space</span>
              <input
                value={form.space}
                onChange={(event) => update("space", event.target.value)}
                placeholder="e.g. ARC-02"
              />
            </label>
            <label className="form-field">
              <span>Business owner</span>
              <select
                value={form.businessOwnerId}
                onChange={(event) =>
                  update("businessOwnerId", event.target.value)
                }
              >
                {owners.map((worker) => (
                  <option value={worker.id} key={worker.id}>
                    {worker.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Business manager *</span>
              <select
                required
                value={form.businessManagerId}
                onChange={(event) =>
                  update("businessManagerId", event.target.value)
                }
              >
                {managers.map((worker) => (
                  <option value={worker.id} key={worker.id}>
                    {worker.name} · {worker.title}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Priority</span>
              <select
                value={form.priority}
                onChange={(event) => update("priority", event.target.value)}
              >
                <option value="normal">Normal</option>
                <option value="high">High</option>
                <option value="low">Low</option>
              </select>
            </label>
          </div>
          <p className="form-owner-note">
            Requested by {ownerName}. The asset is saved before its LAUNCH
            workflow starts.
          </p>
          {error && (
            <div className="inline-error" role="alert">
              {error}
            </div>
          )}
          <div className="dialog-actions">
            <button
              type="button"
              className="button button-secondary"
              onClick={onClose}
              disabled={submitting}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="button button-primary"
              disabled={submitting || managers.length === 0}
            >
              {submitting ? "Saving request…" : "Save asset request"}
            </button>
          </div>
          {managers.length === 0 && (
            <p className="form-error">
              No business managers are available in the API to assign.
            </p>
          )}
        </form>
      </section>
    </div>
  );
}

export default App;
