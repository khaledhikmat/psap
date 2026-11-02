import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
} from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { get, post } from "./api";
import type {
  Asset,
  AssetRequest,
  Baseline,
  Evidence,
  Launch,
  PredefinedAnswer,
  Question,
  QuestionCategory,
  Tier,
  UserAnswerDetail,
  Worker,
  Shield,
} from "./types";

type View = "dashboard" | "asset" | "assessment";
type DemoActor = {
  key: string;
  role: "Business owner" | "Business manager";
  worker: Worker;
};
type Notice = { kind: "success" | "error" | "info"; text: string };

const emptyArray = <T,>(): T[] => [];
const waitingForAnswers = "waiting-4-answers";
const waitingForTierApproval = "waiting-4-tier-approval";
const timedOut = "timedout";
const launchesInReview = new Set(["answers-submitted", "waiting-4-tier-approval"]);
const shieldsInProgress = new Set([
  "waiting-4-answers",
  "answers-submitted",
  "waiting-4-evidences",
  "evidences-submitted",
]);

function mergeLaunchStates(
  refreshed: Launch[],
  previous: Launch[],
): Launch[] {
  const previousById = new Map(previous.map((launch) => [launch.id, launch]));
  return refreshed.map((launch) => {
    const oldLaunch = previousById.get(launch.id);
    if (
      oldLaunch?.state === "answers-submitted" &&
      launch.state === waitingForAnswers
    ) {
      return { ...launch, state: oldLaunch.state };
    }
    return launch;
  });
}

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : "Something went wrong.";
}

function assetStateLabel(state: string): string {
  const labels: Record<string, string> = {
    "pending-launch": "Needs LAUNCH",
    "waiting-4-answers": "Owner input needed",
    "answers-submitted": "Submitted for review",
    "waiting-4-tier-approval": "Awaiting review",
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

function shieldStateLabel(state: string): string {
  const labels: Record<string, string> = {
    "waiting-4-answers": "Answers in progress",
    "answers-submitted": "Answers submitted",
    "waiting-4-evidences": "Evidence in progress",
    "evidences-submitted": "Finalizing",
    completed: "Completed",
    timedout: "Timed out",
    cancelled: "Replaced",
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

function assetDisplayState(
  asset: Asset,
  launches: Launch[],
  isBusinessOwner: boolean,
): string {
  const assetLaunches = launches.filter((launch) => launch.assetId === asset.id);
  const latestLaunch = [...assetLaunches].sort((a, b) =>
    b.createdAt.localeCompare(a.createdAt),
  )[0];
  const activeLaunch = isBusinessOwner
    ? assetLaunches.find((launch) => launch.state === waitingForAnswers)
    : undefined;
  return latestLaunch && launchesInReview.has(latestLaunch.state)
    ? latestLaunch.state
    : activeLaunch?.state ?? asset.state;
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

function safeEvidenceHref(value: string): string | undefined {
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:"
      ? url.toString()
      : undefined;
  } catch {
    return undefined;
  }
}

function App() {
  const navigate = useNavigate();
  const location = useLocation();
  const [assets, setAssets] = useState<Asset[]>([]);
  const [launches, setLaunches] = useState<Launch[]>([]);
  const [shields, setShields] = useState<Shield[]>([]);
  const [workers, setWorkers] = useState<Worker[]>([]);
  const [tiers, setTiers] = useState<Tier[]>([]);
  const [actorKey, setActorKey] = useState("");
  const [view, setView] = useState<View>("dashboard");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [loadError, setLoadError] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [showRequestDialog, setShowRequestDialog] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState<Asset | null>(null);
  const [assetSearch, setAssetSearch] = useState("");
  const [assetLaunches, setAssetLaunches] = useState<Launch[]>([]);
  const [assetShields, setAssetShields] = useState<Shield[]>([]);
  const [assetBaselines, setAssetBaselines] = useState<Baseline[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [activeLaunch, setActiveLaunch] = useState<Launch | null>(null);
  const [activeShield, setActiveShield] = useState<Shield | null>(null);
  const [tierOverride, setTierOverride] = useState("");
  const [evidenceDrafts, setEvidenceDrafts] = useState<
    Record<string, { description: string; url: string }>
  >({});
  const [assessmentType, setAssessmentType] = useState<"launch" | "shield">(
    "launch",
  );
  const [assessmentReadOnly, setAssessmentReadOnly] = useState(false);
  const [evidenceByQuestion, setEvidenceByQuestion] = useState<
    Record<string, Evidence[]>
  >({});
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
  const [shieldSaving, setShieldSaving] = useState(false);
  const [launchError, setLaunchError] = useState("");

  const refreshDashboard = useCallback(async (showLoading = true) => {
    if (showLoading) setIsLoading(true);
    setLoadError("");
    try {
      const [assetRows, launchRows, shieldRows, workerRows, tierRows] = await Promise.all([
        get<Asset[]>("/assets"),
        get<Launch[]>("/launches"),
        get<Shield[]>("/shields"),
        get<Worker[]>("/workers"),
        get<Tier[]>("/tiers"),
      ]);
      setAssets(assetRows ?? emptyArray());
      setLaunches((previous) =>
        mergeLaunchStates(launchRows ?? emptyArray(), previous),
      );
      setShields(shieldRows ?? emptyArray());
      setWorkers(workerRows ?? emptyArray());
      setTiers(tierRows ?? emptyArray());
    } catch (error) {
      setLoadError(errorText(error));
    } finally {
      if (showLoading) setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshDashboard();
  }, [refreshDashboard]);

  useEffect(() => {
    if (view !== "dashboard" || isLoading) return;
    const refreshIfVisible = () => {
      if (document.visibilityState === "visible") {
        void refreshDashboard(false);
      }
    };
    const interval = window.setInterval(refreshIfVisible, 5000);
    window.addEventListener("focus", refreshIfVisible);
    return () => {
      window.clearInterval(interval);
      window.removeEventListener("focus", refreshIfVisible);
    };
  }, [isLoading, refreshDashboard, view]);

  const demoActors = useMemo(() => {
    const actors: DemoActor[] = [];
    for (const worker of workers) {
      if (assets.some((asset) => asset.businessOwner === worker.id)) {
        actors.push({
          key: `owner:${worker.id}`,
          role: "Business owner",
          worker,
        });
      }
      if (assets.some((asset) => asset.businessManager === worker.id)) {
        actors.push({
          key: `manager:${worker.id}`,
          role: "Business manager",
          worker,
        });
      }
    }
    return actors;
  }, [assets, workers]);

  useEffect(() => {
    if (
      demoActors.length > 0 &&
      !demoActors.some((actor) => actor.key === actorKey)
    ) {
      setActorKey(demoActors[0].key);
    }
  }, [actorKey, demoActors]);

  const activeActor = demoActors.find((actor) => actor.key === actorKey);
  const activeWorkerId = activeActor?.worker.id ?? "";
  const isBusinessOwner = activeActor?.role === "Business owner";
  const isBusinessManager = activeActor?.role === "Business manager";

  useEffect(() => {
    document.title = isBusinessManager
      ? "PSAP | Business Manager"
      : "PSAP | Business Owner";
  }, [isBusinessManager]);

  useEffect(() => {
    if (demoActors.length === 0) return;
    const requestedRole =
      location.pathname === "/business-manager"
        ? "Business manager"
        : location.pathname === "/business-owner"
          ? "Business owner"
          : null;
    if (requestedRole === null) {
      const actor = demoActors[0];
      setActorKey(actor.key);
      navigate(
        actor.role === "Business manager"
          ? "/business-manager"
          : "/business-owner",
        { replace: true },
      );
      return;
    }
    const matchingActor = demoActors.find((actor) => actor.role === requestedRole);
    if (!matchingActor) {
      navigate("/business-owner", { replace: true });
      return;
    }
    if (activeActor?.role !== requestedRole) {
      setActorKey(matchingActor.key);
    }
  }, [activeActor?.role, demoActors, location.pathname, navigate]);
  const visibleAssets = useMemo(
    () =>
      assets.filter((asset) =>
        isBusinessOwner
          ? asset.businessOwner === activeWorkerId
          : asset.businessManager === activeWorkerId,
      ),
    [activeWorkerId, assets, isBusinessOwner],
  );
  const visibleAssetIds = useMemo(
    () => new Set(visibleAssets.map((asset) => asset.id)),
    [visibleAssets],
  );
  const visibleLaunches = useMemo(
    () => launches.filter((launch) => visibleAssetIds.has(launch.assetId)),
    [launches, visibleAssetIds],
  );
  const managerReviewQueue = useMemo(
    () =>
      isBusinessManager
        ? visibleLaunches
            .filter((launch) => launch.state === waitingForTierApproval)
            .filter(
              (launch) =>
                !visibleLaunches.some(
                  (other) =>
                    other.assetId === launch.assetId &&
                    other.createdAt > launch.createdAt,
                ),
            )
            .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
        : [],
    [isBusinessManager, visibleLaunches],
  );
  const managerExpiredLaunches = useMemo(
    () =>
      isBusinessManager
        ? visibleLaunches
            .filter((launch) => launch.state === timedOut)
            .filter(
              (launch) =>
                !visibleLaunches.some(
                  (other) =>
                    other.assetId === launch.assetId &&
                    other.createdAt > launch.createdAt,
                ),
            )
            .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
        : [],
    [isBusinessManager, visibleLaunches],
  );
  const managerShieldQueue = useMemo(
    () =>
      isBusinessManager
        ? shields
            .filter(
              (shield) =>
                visibleAssetIds.has(shield.assetId) &&
                shieldsInProgress.has(shield.state),
            )
            .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
        : [],
    [isBusinessManager, shields, visibleAssetIds],
  );
  const pendingLaunches = useMemo(
    () =>
      (isBusinessOwner ? visibleLaunches : [])
        .filter((launch) => launch.state === waitingForAnswers)
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt)),
    [isBusinessOwner, visibleLaunches],
  );
  const ownerLaunchesInReview = useMemo(
    () =>
      isBusinessOwner
        ? visibleLaunches.filter((launch) =>
            launchesInReview.has(launch.state),
          )
        : [],
    [isBusinessOwner, visibleLaunches],
  );
  const ownerOptions = useMemo(
    () =>
      workers.filter((worker) =>
        assets.some((asset) => asset.businessOwner === worker.id),
      ),
    [assets, workers],
  );
  const managerOptions = useMemo(() => {
    return workers.filter((worker) =>
      assets.some((asset) => asset.businessManager === worker.id),
    );
  }, [assets, workers]);
  const owner = activeActor?.worker;
  const tierById = useMemo(
    () => new Map(tiers.map((tier) => [tier.id, tier])),
    [tiers],
  );
  const filteredAssets = useMemo(() => {
    const query = assetSearch.trim().toLowerCase();
    if (!query) return visibleAssets;
    return visibleAssets.filter((asset) => {
      const tier = asset.tierId ? tierById.get(asset.tierId) : undefined;
      const status = assetStateLabel(
        assetDisplayState(asset, visibleLaunches, isBusinessOwner),
      );
      return [
        asset.name,
        asset.id,
        asset.campus,
        asset.building,
        asset.section,
        asset.level,
        asset.wing,
        asset.floor,
        asset.space,
        asset.priority,
        locationLabel(asset),
        tier?.name,
        tier?.title,
        status,
      ]
        .filter((value): value is string => Boolean(value))
        .some((value) => value.toLowerCase().includes(query));
    });
  }, [assetSearch, isBusinessOwner, tierById, visibleAssets, visibleLaunches]);

  async function loadAsset(asset: Asset) {
    setSelectedAsset(asset);
    setView("asset");
    setDetailLoading(true);
    setNotice(null);
    try {
      const [history, shields, baselines] = await Promise.all([
        get<Launch[]>(`/assets/${encodeURIComponent(asset.id)}/launches`),
        get<Shield[]>(`/assets/${encodeURIComponent(asset.id)}/shields`),
        get<Baseline[]>(`/assets/${encodeURIComponent(asset.id)}/baselines`),
      ]);
      setAssetLaunches(
        mergeLaunchStates(history ?? [], launches).sort((a, b) =>
          b.createdAt.localeCompare(a.createdAt),
        ),
      );
      setAssetShields(
        (shields ?? []).sort((a, b) =>
          b.createdAt.localeCompare(a.createdAt),
        ),
      );
      setAssetBaselines(baselines ?? []);
    } catch (error) {
      setNotice({ kind: "error", text: errorText(error) });
      setAssetLaunches([]);
      setAssetShields([]);
      setAssetBaselines([]);
    } finally {
      setDetailLoading(false);
    }
  }

  async function loadLaunch(launch: Launch, readOnly = false) {
    setActiveLaunch(null);
    setActiveShield(null);
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
      setTierOverride("");
      setAssessmentType("launch");
      setAssessmentReadOnly(readOnly);
      setEvidenceByQuestion({});
      setCategories(orderedCategories);
      setQuestions(orderedQuestions);
      setPredefinedAnswers(orderedAnswers);
      setLaunchAnswers(initial);
      setSavedAnswers(initial);
      setCategoryIndex(0);
      setView("assessment");
    } catch (error) {
      setLaunchError(errorText(error));
      setView("assessment");
    } finally {
      setLaunchLoading(false);
    }
  }

  async function loadShield(shield: Shield, readOnly = true) {
    setActiveLaunch(null);
    setActiveShield(null);
    setLaunchLoading(true);
    setLaunchError("");
    setNotice(null);
    try {
      const [categoryRows, questionRows, answerRows, savedRows, evidenceRows] =
        await Promise.all([
          get<QuestionCategory[]>("/question-categories?type=shield"),
          get<Question[]>("/questions?categoryType=shield"),
          get<PredefinedAnswer[]>("/predefined-answers?type=shield"),
          get<UserAnswerDetail[]>(
            `/shields/${encodeURIComponent(shield.id)}/answers`,
          ),
          get<Evidence[]>(
            `/shields/${encodeURIComponent(shield.id)}/evidences`,
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
      const evidencesByQuestion: Record<string, Evidence[]> = {};
      const evidenceDraftsByQuestion: Record<
        string,
        { description: string; url: string }
      > = {};
      for (const evidence of evidenceRows ?? []) {
        evidencesByQuestion[evidence.questionId] ??= [];
        evidencesByQuestion[evidence.questionId].push(evidence);
        evidenceDraftsByQuestion[evidence.questionId] = {
          description: evidence.description,
          url: evidence.url,
        };
      }
      setActiveShield(shield);
      setAssessmentType("shield");
      setAssessmentReadOnly(
        readOnly ||
          !isBusinessManager ||
          !["waiting-4-answers", "waiting-4-evidences"].includes(shield.state),
      );
      setCategories(orderedCategories);
      setQuestions(orderedQuestions);
      setPredefinedAnswers(orderedAnswers);
      setLaunchAnswers(initial);
      setSavedAnswers(initial);
      setEvidenceByQuestion(evidencesByQuestion);
      setEvidenceDrafts(evidenceDraftsByQuestion);
      setCategoryIndex(0);
      setView("assessment");
    } catch (error) {
      setLaunchError(errorText(error));
      setView("assessment");
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

  async function openManagerReview(launch: Launch) {
    const asset = assets.find((row) => row.id === launch.assetId);
    if (!asset) {
      setNotice({
        kind: "error",
        text: "The asset for this LAUNCH could not be found.",
      });
      return;
    }
    await loadAsset(asset);
    await loadLaunch(launch, true);
  }

  async function beginShield(asset: Asset) {
    const previousShieldId = shields
      .filter((shield) => shield.assetId === asset.id)
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0]?.id;
    setNotice({ kind: "info", text: "Starting a SHIELD assessment…" });
    try {
      await post("/shields", { assetId: asset.id });
      const deadline = Date.now() + 8000;
      while (Date.now() < deadline) {
        await new Promise((resolve) => window.setTimeout(resolve, 400));
        const history = await get<Shield[]>(
          `/assets/${encodeURIComponent(asset.id)}/shields`,
        );
        const createdShield = (history ?? [])
          .filter((shield) => shield.id !== previousShieldId)
          .sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0];
        if (createdShield) {
          await refreshDashboard();
          await loadAsset(asset);
          await loadShield(createdShield, false);
          setNotice({
            kind: "success",
            text: "SHIELD is ready. Record the observed exposure and supporting evidence.",
          });
          return;
        }
      }
      await refreshDashboard();
      setNotice({
        kind: "info",
        text: "The SHIELD request was accepted. It will appear when the workflow worker creates it.",
      });
    } catch (error) {
      setNotice({
        kind: "error",
        text: `SHIELD could not be started: ${errorText(error)}`,
      });
    }
  }

  async function validateLaunchTier() {
    if (!activeLaunch || !isBusinessManager) return;
    setLaunchSaving(true);
    setLaunchError("");
    try {
      await post(
        `/launches/${encodeURIComponent(activeLaunch.id)}/validate-tier`,
        { tierId: tierOverride },
      );
      const validated = {
        ...activeLaunch,
        state: "tier-validated",
        ...(tierOverride ? { tierId: tierOverride } : {}),
      };
      setLaunches((current) =>
        current.map((launch) =>
          launch.id === activeLaunch.id ? validated : launch,
        ),
      );
      setAssetLaunches((current) =>
        current.map((launch) =>
          launch.id === activeLaunch.id ? validated : launch,
        ),
      );
      setActiveLaunch(null);
      setView("dashboard");
      setNotice({
        kind: "success",
        text: "LAUNCH tier validated. The assessment is now in the asset record.",
      });
    } catch (error) {
      setLaunchError(errorText(error));
    } finally {
      setLaunchSaving(false);
    }
  }

  async function saveShieldAnswers(finalize = false) {
    if (!activeShield || !isBusinessManager || assessmentReadOnly) return;
    const unanswered = questions.filter(
      (question) => (launchAnswers[question.id] ?? []).length === 0,
    ).length;
    if (finalize && unanswered > 0) {
      setLaunchError(
        `Answer all ${questions.length} SHIELD questions before continuing. ${unanswered} remain unanswered.`,
      );
      return;
    }
    const submissions = questions
      .filter((question) => (launchAnswers[question.id] ?? []).length > 0)
      .map((question) => ({
        questionId: question.id,
        predefinedAnswerIds: launchAnswers[question.id],
      }));
    setShieldSaving(true);
    setLaunchError("");
    try {
      await post(
        `/shields/${encodeURIComponent(activeShield.id)}/${
          finalize ? "finalize-answers" : "submit-answers"
        }`,
        { answers: submissions },
      );
      setSavedAnswers(launchAnswers);
      if (finalize) {
        setActiveShield((current) =>
          current ? { ...current, state: "waiting-4-evidences" } : current,
        );
        setNotice({
          kind: "success",
          text: "SHIELD answers saved. Add evidence for each required question.",
        });
      } else {
        setNotice({ kind: "success", text: "SHIELD answers saved." });
      }
    } catch (error) {
      setLaunchError(errorText(error));
    } finally {
      setShieldSaving(false);
    }
  }

  async function saveShieldEvidences(finalize = false) {
    if (!activeShield || !isBusinessManager || assessmentReadOnly) return;
    const evidences = questions
      .map((question) => ({
        questionId: question.id,
        description: evidenceDrafts[question.id]?.description.trim() ?? "",
        url: evidenceDrafts[question.id]?.url.trim() ?? "",
      }))
      .filter((evidence) => evidence.description || evidence.url);
    const hasInvalidEvidenceUrl = evidences.some(
      (evidence) => evidence.url && !safeEvidenceHref(evidence.url),
    );
    if (hasInvalidEvidenceUrl) {
      setLaunchError("Evidence links must be valid HTTP or HTTPS URLs.");
      return;
    }
    const missingEvidence = questions.filter(
      (question) =>
        question.evidenceRequired &&
        !evidenceDrafts[question.id]?.description.trim() &&
        !evidenceDrafts[question.id]?.url.trim(),
    ).length;
    if (finalize && missingEvidence > 0) {
      setLaunchError(
        `Add evidence for each required question. ${missingEvidence} still need evidence.`,
      );
      return;
    }
    setShieldSaving(true);
    setLaunchError("");
    try {
      await post(
        `/shields/${encodeURIComponent(activeShield.id)}/${
          finalize ? "finalize-evidences" : "submit-evidences"
        }`,
        { evidences },
      );
      if (finalize) {
        const submitted = { ...activeShield, state: "evidences-submitted" };
        setActiveShield(submitted);
        setShields((current) =>
          current.map((shield) =>
            shield.id === submitted.id ? submitted : shield,
          ),
        );
        setAssetShields((current) =>
          current.map((shield) =>
            shield.id === submitted.id ? submitted : shield,
          ),
        );
        setAssessmentReadOnly(true);
        setNotice({
          kind: "success",
          text: "SHIELD evidence submitted. The assessment is being finalized.",
        });
      } else {
        setNotice({ kind: "success", text: "SHIELD evidence saved." });
      }
    } catch (error) {
      setLaunchError(errorText(error));
    } finally {
      setShieldSaving(false);
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
    if (!activeLaunch || assessmentReadOnly) return;
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
        setLaunches((current) =>
          current.map((launch) =>
            launch.id === activeLaunch.id
              ? { ...launch, state: "answers-submitted" }
              : launch,
          ),
        );
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
    setActiveShield(null);
    setNotice(null);
  }

  function backToAssetDetails() {
    setView("asset");
    setActiveLaunch(null);
    setActiveShield(null);
    setNotice(null);
  }

  const category = categories[categoryIndex];
  const categoryQuestions = category
    ? questions.filter((question) => question.categoryId === category.id)
    : [];
  const unansweredCount = questions.filter(
    (question) => (launchAnswers[question.id] ?? []).length === 0,
  ).length;
  const latestAssetLaunch = assetLaunches[0];
  const assetDetailState =
    latestAssetLaunch &&
    (latestAssetLaunch.state === waitingForAnswers ||
      launchesInReview.has(latestAssetLaunch.state))
      ? latestAssetLaunch.state
      : selectedAsset?.state;
  const managerCanValidate =
    isBusinessManager &&
    activeLaunch !== null &&
    activeLaunch.state === waitingForTierApproval;

  useEffect(() => {
    if (view !== "assessment" || categories.length < 2) return;
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
              aria-label="Select demo user and role"
              value={actorKey}
              onChange={(event) => {
                const nextActor = demoActors.find(
                  (actor) => actor.key === event.target.value,
                );
                setActorKey(event.target.value);
                navigate(
                  nextActor?.role === "Business manager"
                    ? "/business-manager"
                    : "/business-owner",
                );
                setView("dashboard");
                setSelectedAsset(null);
                setActiveLaunch(null);
                setActiveShield(null);
              }}
              disabled={demoActors.length === 0}
            >
              {demoActors.map((actor) => (
                <option key={actor.key} value={actor.key}>
                  {actor.worker.name} · {actor.role}
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
            <button
              className="button button-secondary"
              onClick={() => void refreshDashboard()}
            >
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
                <div className="eyebrow">
                  {isBusinessOwner ? "BUSINESS OWNER PORTAL" : "BUSINESS MANAGER PORTAL"}
                </div>
                <h1 id="dashboard-title">
                  Good to see you, {owner?.name ?? "User"}
                </h1>
                <p>
                  {isBusinessOwner
                    ? "Manage your secure spaces and keep their business assessments up to date."
                    : "Validate submitted LAUNCH assessments and assess observable SHIELD exposure for assigned assets."}
                </p>
              </div>
              {isBusinessOwner && (
                <button
                  className="button button-primary"
                  onClick={() => setShowRequestDialog(true)}
                  disabled={workers.length === 0}
                >
                  <span className="button-plus">＋</span> Request a new asset
                </button>
              )}
            </div>

            <div className="metric-grid">
              <Metric
                label={isBusinessOwner ? "Assets you own" : "Assets you manage"}
                value={visibleAssets.length}
                caption={
                  isBusinessOwner
                    ? "Secure rooms and areas"
                    : "Assigned secure rooms and areas"
                }
                accent="blue"
              />
              <Metric
                label={isBusinessOwner ? "LAUNCH actions" : "LAUNCH reviews"}
                value={
                  isBusinessOwner
                    ? pendingLaunches.length
                    : managerReviewQueue.length
                }
                caption={
                  isBusinessOwner
                    ? pendingLaunches.length > 0
                      ? "Waiting for your input"
                      : ownerLaunchesInReview.length > 0
                        ? `${ownerLaunchesInReview.length} submitted for review`
                        : "No owner action needed"
                    : managerReviewQueue.length > 0
                      ? "Awaiting tier validation"
                      : "No LAUNCH reviews waiting"
                }
                accent={
                  (isBusinessOwner
                    ? pendingLaunches.length
                    : managerReviewQueue.length) > 0
                    ? "amber"
                    : "green"
                }
              />
              <Metric
                label={isBusinessManager ? "SHIELD in progress" : "Tiered assets"}
                value={
                  isBusinessManager
                    ? managerShieldQueue.length
                    : visibleAssets.filter((asset) => asset.tierId).length
                }
                caption={
                  isBusinessManager
                    ? "Continue exposure assessments"
                    : "Validated by security"
                }
                accent="navy"
              />
            </div>

            <div className="dashboard-grid">
              <section className="panel work-panel" aria-labelledby="work-title">
                <div className="panel-heading">
                  <div>
                    <div className="eyebrow">YOUR NEXT STEP</div>
                    <h2 id="work-title">
                      {isBusinessOwner ? "LAUNCH tasks" : "Manager work queue"}
                    </h2>
                  </div>
                  <span className="count-badge">
                    {isBusinessOwner
                      ? pendingLaunches.length
                      : managerReviewQueue.length +
                        managerShieldQueue.length +
                        managerExpiredLaunches.length}
                  </span>
                </div>
                {isBusinessManager ? (
                  managerReviewQueue.length === 0 &&
                  managerShieldQueue.length === 0 &&
                  managerExpiredLaunches.length === 0 ? (
                    <div className="empty-state compact-empty">
                      <span className="empty-icon">✓</span>
                      <strong>You’re all caught up</strong>
                      <span>
                        Submitted LAUNCH assessments and active SHIELD work will
                        appear here.
                      </span>
                    </div>
                  ) : (
                    <div className="task-list">
                      {managerReviewQueue.map((launch) => {
                        const asset = assets.find(
                          (row) => row.id === launch.assetId,
                        );
                        return (
                          <article className="task-card" key={launch.id}>
                            <div className="task-icon" aria-hidden="true">L</div>
                            <div className="task-copy">
                              <strong>{asset?.name ?? launch.assetId}</strong>
                              <span>
                                {asset
                                  ? locationLabel(asset)
                                  : "Asset details loading"}
                              </span>
                              <small>
                                Submitted {formatDate(launch.updatedAt)}
                              </small>
                            </div>
                            <button
                              className="button button-small button-primary"
                              onClick={() => void openManagerReview(launch)}
                            >
                              Review LAUNCH <span aria-hidden="true">→</span>
                            </button>
                          </article>
                        );
                      })}
                      {managerShieldQueue.map((shield) => {
                        const asset = assets.find(
                          (row) => row.id === shield.assetId,
                        );
                        return (
                          <article className="task-card" key={shield.id}>
                            <div className="task-icon shield-task-icon" aria-hidden="true">S</div>
                            <div className="task-copy">
                              <strong>{asset?.name ?? shield.assetId}</strong>
                              <span>
                                {asset
                                  ? locationLabel(asset)
                                  : "Asset details loading"}
                              </span>
                              <small>
                                SHIELD · {shieldStateLabel(shield.state)}
                              </small>
                            </div>
                            <button
                              className="button button-small button-primary"
                              onClick={() => {
                                if (asset) {
                                  void loadAsset(asset).then(() =>
                                    loadShield(shield, false),
                                  );
                                }
                              }}
                              disabled={!asset}
                            >
                              Continue SHIELD <span aria-hidden="true">→</span>
                            </button>
                          </article>
                        );
                      })}
                      {managerExpiredLaunches.map((launch) => {
                        const asset = assets.find(
                          (row) => row.id === launch.assetId,
                        );
                        return (
                          <article className="task-card expired-task-card" key={launch.id}>
                            <div className="task-icon expired-task-icon" aria-hidden="true">!</div>
                            <div className="task-copy">
                              <strong>{asset?.name ?? launch.assetId}</strong>
                              <span>
                                {asset
                                  ? locationLabel(asset)
                                  : "Asset details loading"}
                              </span>
                              <small>
                                LAUNCH timed out · Ask the owner to request a
                                reassessment
                              </small>
                            </div>
                            <button
                              className="button button-small button-secondary"
                              onClick={() => void openManagerReview(launch)}
                              disabled={!asset}
                            >
                              View record <span aria-hidden="true">→</span>
                            </button>
                          </article>
                        );
                      })}
                    </div>
                  )
                ) : pendingLaunches.length === 0 ? (
                  <div className="empty-state compact-empty">
                    <span className="empty-icon">✓</span>
                    <strong>
                      {ownerLaunchesInReview.length > 0
                        ? "Your LAUNCH is in review"
                        : isBusinessOwner
                          ? "You’re all caught up"
                          : "No assessments awaiting your review"}
                    </strong>
                    <span>
                      {ownerLaunchesInReview.length > 0
                        ? "Your business manager or security assessor will review your submitted answers."
                        : isBusinessOwner
                          ? "No LAUNCH questionnaires need your input."
                          : "You can open an asset to view its LAUNCH and SHIELD history."}
                    </span>
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
                <div className="eyebrow">
                  {isBusinessManager ? "MANAGER RESPONSIBILITIES" : "A QUICK REMINDER"}
                </div>
                <h2>
                  {isBusinessManager
                    ? "Validate business impact, then assess exposure"
                    : "LAUNCH is about business impact"}
                </h2>
                <p>
                  {isBusinessManager
                    ? "Check submitted answers against the available evidence. Confirm or override the tier, then record SHIELD conditions with evidence."
                    : "Describe what the room supports and what would happen if it were unavailable. Security will validate your answers and confirm the tier."}
                </p>
                <div className="principle-rule">
                  {isBusinessManager
                    ? "SHIELD documents observable exposure. It does not change the validated tier."
                    : "You won’t be asked to choose cameras, alarms, or other security controls."}
                </div>
              </aside>
            </div>

            <section className="assets-section" aria-labelledby="assets-title">
              <div className="section-heading">
                <div>
                  <div className="eyebrow">YOUR PORTFOLIO</div>
                  <h2 id="assets-title">
                    {isBusinessOwner ? "Assets you own" : "Assets you manage"}
                  </h2>
                </div>
              </div>
              {visibleAssets.length === 0 ? (
                <div className="panel empty-assets">
                  <div className="empty-icon">⌂</div>
                  <h3>
                    {isBusinessOwner
                      ? "No assets assigned yet"
                      : "No assets assigned to you"}
                  </h3>
                  <p>
                    {isBusinessOwner
                      ? "Request a new secure room or area to begin its LAUNCH assessment."
                      : "Assets assigned to you as business manager will appear here."}
                  </p>
                  {isBusinessOwner && (
                    <button
                      className="button button-primary"
                      onClick={() => setShowRequestDialog(true)}
                    >
                      Request an asset
                    </button>
                  )}
                </div>
              ) : (
                <>
                  <div className="asset-toolbar">
                    <label className="asset-search">
                      <span>Search assets</span>
                      <input
                        type="search"
                        value={assetSearch}
                        onChange={(event) => setAssetSearch(event.target.value)}
                        placeholder="Name, location, tier, or status"
                      />
                    </label>
                    <span className="muted" aria-live="polite">
                      Showing {filteredAssets.length} of {visibleAssets.length}{" "}
                      {visibleAssets.length === 1 ? "asset" : "assets"}
                    </span>
                  </div>
                  {filteredAssets.length === 0 ? (
                    <div className="panel asset-search-empty">
                      <strong>No matching assets</strong>
                      <span>Try a different name, location, tier, or status.</span>
                    </div>
                  ) : (
                    <div className="asset-table-wrap">
                      <table
                        className="asset-table"
                        aria-label="Assets assigned to your selected role"
                      >
                        <thead>
                          <tr>
                            <th scope="col">Asset</th>
                            <th scope="col">Location</th>
                            <th scope="col">Tier</th>
                            <th scope="col">Status</th>
                            <th scope="col">Action</th>
                          </tr>
                        </thead>
                        <tbody>
                  {filteredAssets.map((asset) => {
                    const assetLaunchesForCurrent = visibleLaunches.filter(
                      (launch) => launch.assetId === asset.id,
                    );
                    const active = isBusinessOwner
                      ? assetLaunchesForCurrent.find(
                      (launch) =>
                        launch.state === waitingForAnswers,
                      )
                      : undefined;
                    const latestLaunch = assetLaunchesForCurrent
                      .sort((a, b) =>
                        b.createdAt.localeCompare(a.createdAt),
                      )[0];
                    const activeShield = isBusinessManager
                      ? shields
                          .filter(
                            (shield) =>
                              shield.assetId === asset.id &&
                              shieldsInProgress.has(shield.state),
                          )
                          .sort((a, b) =>
                            b.createdAt.localeCompare(a.createdAt),
                          )[0]
                      : undefined;
                    const awaitingReview =
                      latestLaunch && launchesInReview.has(latestLaunch.state);
                    const displayState = assetDisplayState(
                      asset,
                      visibleLaunches,
                      isBusinessOwner,
                    );
                    const tier = asset.tierId
                      ? tierById.get(asset.tierId)
                      : undefined;
                    return (
                      <tr key={asset.id}>
                        <th scope="row">
                          <button
                            className="asset-table-name"
                            onClick={() => void loadAsset(asset)}
                          >
                            {asset.name}
                          </button>
                          <span className="asset-table-id">{asset.id}</span>
                        </th>
                        <td>{locationLabel(asset) || "—"}</td>
                        <td>
                          {tier ? (
                            <>
                              <span className="tier-dot" />
                              {tier.name} · {tier.title}
                            </>
                          ) : (
                            <span className="muted">Tier not assigned</span>
                          )}
                        </td>
                        <td>
                          <span
                            className={`state-pill state-${displayState}`}
                          >
                            {assetStateLabel(displayState)}
                          </span>
                        </td>
                        <td>
                          {active ? (
                            <button
                              className="asset-table-action"
                              onClick={() => void loadLaunch(active)}
                            >
                              Continue LAUNCH <span aria-hidden="true">→</span>
                            </button>
                          ) : isBusinessManager && activeShield ? (
                            <button
                              className="asset-table-action"
                              onClick={() => {
                                void loadAsset(asset).then(() =>
                                  loadShield(activeShield, false),
                                );
                              }}
                            >
                              Continue SHIELD <span aria-hidden="true">→</span>
                            </button>
                          ) : isBusinessManager ? (
                            latestLaunch?.state === "tier-validated" ||
                            asset.state === "pending-shield" ||
                            asset.state === "shielded" ? (
                              <button
                                className="asset-table-action"
                                onClick={() => void beginShield(asset)}
                              >
                                Start SHIELD <span aria-hidden="true">→</span>
                              </button>
                            ) : latestLaunch?.state === waitingForTierApproval ? (
                              <button
                                className="asset-table-action"
                                onClick={() => void openManagerReview(latestLaunch)}
                              >
                                Review LAUNCH <span aria-hidden="true">→</span>
                              </button>
                            ) : (
                              <button
                                className="asset-table-action"
                                onClick={() => void loadAsset(asset)}
                              >
                                View history <span aria-hidden="true">→</span>
                              </button>
                            )
                          ) : !isBusinessOwner ? (
                            <button
                              className="asset-table-action"
                              onClick={() => void loadAsset(asset)}
                            >
                              View history <span aria-hidden="true">→</span>
                            </button>
                          ) : awaitingReview ? (
                            <span className="asset-table-note">In review</span>
                          ) : (
                            <button
                              className="asset-table-action"
                              onClick={() => void beginLaunch(asset)}
                            >
                              {latestLaunch ? "Request reassessment" : "Start LAUNCH"}{" "}
                              <span aria-hidden="true">→</span>
                            </button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </>
              )}
            </section>

            <footer className="page-footer">
              <span>PSAP · Secure room and area assessment</span>
              <button
                className="text-button"
                onClick={() => void refreshDashboard()}
              >
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
                {isBusinessOwner &&
                latestAssetLaunch?.state === waitingForAnswers ? (
                  <button
                    className="button button-primary"
                    onClick={() => void loadLaunch(latestAssetLaunch)}
                  >
                    Continue LAUNCH
                  </button>
                ) : latestAssetLaunch &&
                  launchesInReview.has(latestAssetLaunch.state) ? (
                  <button className="button button-secondary" disabled>
                    Awaiting review
                  </button>
                ) : isBusinessOwner ? (
                  <button
                    className="button button-primary"
                    onClick={() => void beginLaunch(selectedAsset)}
                  >
                    {assetLaunches.length === 0
                      ? "Start LAUNCH"
                      : "Request reassessment"}
                  </button>
                ) : isBusinessManager ? (
                  (() => {
                    const activeShield = assetShields.find((shield) =>
                      shieldsInProgress.has(shield.state),
                    );
                    const canStartShield =
                      latestAssetLaunch?.state === "tier-validated" ||
                      selectedAsset.state === "pending-shield" ||
                      selectedAsset.state === "shielded";
                    return activeShield ? (
                      <button
                        className="button button-primary"
                        onClick={() =>
                          void loadShield(activeShield, false)
                        }
                      >
                        Continue SHIELD
                      </button>
                    ) : canStartShield ? (
                      <button
                        className="button button-primary"
                        onClick={() => void beginShield(selectedAsset)}
                      >
                        Start SHIELD
                      </button>
                    ) : null;
                  })()
                ) : null}
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
                      className={`state-pill state-${assetDetailState}`}
                    >
                      {assetStateLabel(assetDetailState ?? selectedAsset.state)}
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
                          {launch.state === waitingForAnswers &&
                          isBusinessOwner ? (
                            <button
                              className="text-button"
                              onClick={() => void loadLaunch(launch)}
                            >
                              Continue
                            </button>
                          ) : (
                            <button
                              className="text-button"
                              onClick={() =>
                                void loadLaunch(
                                  launch,
                                  launch.state !== waitingForAnswers ||
                                    !isBusinessOwner,
                                )
                              }
                            >
                              View answers
                            </button>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                <div className="panel history-panel">
                  <div className="panel-heading">
                    <div>
                      <div className="eyebrow">EXPOSURE RECORD</div>
                      <h2>SHIELD history</h2>
                    </div>
                    <span className="muted">
                      {assetShields.length}{" "}
                      {assetShields.length === 1 ? "assessment" : "assessments"}
                    </span>
                  </div>
                  {assetShields.length === 0 ? (
                    <p className="muted">
                      No SHIELD assessments have been started for this asset.
                    </p>
                  ) : (
                    <div className="history-list">
                      {assetShields.map((shield) => (
                        <div className="history-row" key={shield.id}>
                          <div className="history-marker shield-marker" aria-hidden="true" />
                          <div className="history-copy">
                            <strong>
                              {shieldStateLabel(shield.state)}
                            </strong>
                            <span>
                              Started {formatDate(shield.createdAt)}
                            </span>
                          </div>
                          <span className="muted">
                            {shield.score > 0
                              ? `Exposure score ${shield.score} / 45`
                              : "Score pending"}
                          </span>
                          <button
                            className="text-button"
                            onClick={() =>
                              void loadShield(
                                shield,
                                !(isBusinessManager &&
                                  shieldsInProgress.has(shield.state)),
                              )
                            }
                          >
                            {isBusinessManager &&
                            shieldsInProgress.has(shield.state)
                              ? "Continue SHIELD"
                              : "View SHIELD"}
                          </button>
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
                    <div className="eyebrow">
                      {isBusinessOwner ? "BUSINESS OWNER GUIDE" : "ASSESSMENT GUIDE"}
                    </div>
                    <h2>{isBusinessOwner ? "What to expect" : "Manager review"}</h2>
                    {isBusinessOwner ? (
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
                    ) : (
                      <ul className="guide-list">
                        <li>Verify business answers against available evidence.</li>
                        <li>Confirm the calculated tier or assign an override.</li>
                        <li>Record SHIELD exposure with supporting evidence.</li>
                        <li>SHIELD does not change the validated tier.</li>
                      </ul>
                    )}
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

        {view === "assessment" && (
          <section className="launch-view" aria-labelledby="assessment-title">
            <button className="back-link" onClick={backToAssetDetails}>
              <span aria-hidden="true">←</span> Back to asset details
            </button>
            {launchLoading ? (
              <div className="loading-state">
                <span className="spinner" /> Loading assessment…
              </div>
            ) : launchError && !activeLaunch && !activeShield ? (
              <div className="error-state" role="alert">
                <div>
                  <strong>Could not open this assessment</strong>
                  <p>{launchError}</p>
                </div>
                <button className="button button-secondary" onClick={goToDashboard}>
                  Return to overview
                </button>
              </div>
            ) : activeLaunch || activeShield ? (
              <>
                <div className="page-heading launch-heading">
                  <div>
                    <div className="eyebrow">
                      {assessmentType === "launch"
                        ? managerCanValidate
                          ? "LAUNCH VALIDATION"
                          : "BUSINESS IMPACT ASSESSMENT"
                        : isBusinessManager
                          ? "MANAGER SHIELD ASSESSMENT"
                          : "SECURITY EXPOSURE ASSESSMENT"}
                      {assessmentReadOnly ? " · READ ONLY" : ""}
                    </div>
                    <h1 id="assessment-title">
                      {assets.find(
                        (asset) =>
                          asset.id === (activeLaunch?.assetId ?? activeShield?.assetId),
                      )?.name ??
                        (assessmentType === "launch"
                          ? "LAUNCH questionnaire"
                          : "SHIELD questionnaire")}
                    </h1>
                    <p>
                      {managerCanValidate
                        ? "Review the submitted answers and confirm the calculated tier or select an override."
                        : assessmentReadOnly
                        ? `Viewing saved ${assessmentType.toUpperCase()} answers${activeLaunch ? ` · ${launchStateLabel(activeLaunch.state)}` : activeShield ? ` · ${shieldStateLabel(activeShield.state)}` : ""}. This historical assessment cannot be changed.`
                        : assessmentType === "shield"
                          ? activeShield?.state === "waiting-4-evidences"
                            ? "Record evidence for each SHIELD rating, then submit to finalize the exposure score."
                            : "Select the highest exposure condition actually observed for each question."
                          : "Answer for the actual room or area. Your business manager or security assessor will validate the result."}
                    </p>
                    {activeShield?.state === "evidences-submitted" ? (
                      <p className="assessment-score">
                        SHIELD exposure score is being finalized.
                      </p>
                    ) : activeShield &&
                      (activeShield.score > 0 ||
                        activeShield.state === "completed") ? (
                      <p className="assessment-score">
                        SHIELD exposure score:{" "}
                        <strong>{activeShield.score} / 45</strong>
                      </p>
                    ) : null}
                  </div>
                  {managerCanValidate ? (
                    <span className="readonly-indicator">Manager review</span>
                  ) : assessmentReadOnly ? (
                    <span className="readonly-indicator">
                      Read-only record
                    </span>
                  ) : (
                    <span className="draft-indicator">
                      <span className="draft-dot" />
                      {Object.keys(savedAnswers).length > 0
                        ? "Saved progress"
                        : "Not yet saved"}
                    </span>
                  )}
                </div>

                <div className="launch-layout">
                  <aside className="step-panel" aria-label="Question categories">
                    <div className="step-panel-title">
                      {assessmentType.toUpperCase()} CATEGORIES
                    </div>
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
                    {!assessmentReadOnly && (
                      <div className="step-help">
                      <strong>Keyboard tip</strong>
                      <span>
                        Use Alt + ← / → to move between categories.
                      </span>
                      </div>
                    )}
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
                            {assessmentType === "launch"
                              ? assessmentReadOnly
                                ? "Recorded business answer"
                                : "Business owner input"
                              : "Recorded exposure rating"}
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
                                        } ${
                                          assessmentReadOnly
                                            ? "read-only-option"
                                            : ""
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
                                          disabled={assessmentReadOnly}
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
                                {assessmentType === "shield" && (
                                  !assessmentReadOnly &&
                                  isBusinessManager &&
                                  activeShield?.state ===
                                    "waiting-4-evidences" ? (
                                    <div className="evidence-editor">
                                      <strong>
                                        Evidence
                                        {question.evidenceRequired
                                          ? " · Required"
                                          : " · Optional"}
                                      </strong>
                                      <label>
                                        <span>Description or source note</span>
                                        <input
                                          value={
                                            evidenceDrafts[question.id]
                                              ?.description ?? ""
                                          }
                                          onChange={(event) =>
                                            setEvidenceDrafts((current) => ({
                                              ...current,
                                              [question.id]: {
                                                description: event.target.value,
                                                url:
                                                  current[question.id]?.url ??
                                                  "",
                                              },
                                            }))
                                          }
                                          placeholder="For example, site floor plan or timed walk"
                                        />
                                      </label>
                                      <label>
                                        <span>Evidence URL</span>
                                        <input
                                          type="url"
                                          value={
                                            evidenceDrafts[question.id]?.url ??
                                            ""
                                          }
                                          onChange={(event) =>
                                            setEvidenceDrafts((current) => ({
                                              ...current,
                                              [question.id]: {
                                                description:
                                                  current[question.id]
                                                    ?.description ?? "",
                                                url: event.target.value,
                                              },
                                            }))
                                          }
                                          placeholder="https://…"
                                        />
                                      </label>
                                    </div>
                                  ) : activeShield?.state ===
                                    "waiting-4-answers" ? (
                                    <div className="evidence-record">
                                      {question.evidenceRequired
                                        ? "Evidence will be recorded after all exposure ratings are saved."
                                        : "Evidence is optional for this question."}
                                    </div>
                                  ) : (
                                    <div className="evidence-record">
                                      <strong>Recorded evidence</strong>
                                      {(evidenceByQuestion[question.id] ?? [])
                                        .length === 0 ? (
                                        evidenceDrafts[question.id]
                                          ?.description ||
                                        evidenceDrafts[question.id]?.url ? (
                                          <div className="evidence-item">
                                            <span>
                                              {evidenceDrafts[question.id]
                                                ?.description ||
                                                "Evidence source"}
                                            </span>
                                            {evidenceDrafts[question.id]?.url &&
                                              safeEvidenceHref(
                                                evidenceDrafts[question.id].url,
                                              ) && (
                                                <a
                                                  href={safeEvidenceHref(
                                                    evidenceDrafts[question.id]
                                                      .url,
                                                  )}
                                                  target="_blank"
                                                  rel="noreferrer"
                                                >
                                                  View evidence source
                                                </a>
                                              )}
                                          </div>
                                        ) : (
                                          <span>No evidence was recorded.</span>
                                        )
                                      ) : (
                                        (
                                          evidenceByQuestion[question.id] ?? []
                                        ).map((evidence) => (
                                          <div
                                            className="evidence-item"
                                            key={evidence.id}
                                          >
                                            <span>
                                              {evidence.description ||
                                                "Evidence source"}
                                            </span>
                                            {evidence.url &&
                                              safeEvidenceHref(evidence.url) && (
                                                <a
                                                  href={safeEvidenceHref(
                                                    evidence.url,
                                                  )}
                                                  target="_blank"
                                                  rel="noreferrer"
                                                >
                                                  View evidence source
                                                </a>
                                              )}
                                          </div>
                                        ))
                                      )}
                                    </div>
                                  )
                                )}
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
                            {assessmentReadOnly
                              ? `${questions.length - unansweredCount} of ${questions.length} questions answered`
                              : `${unansweredCount} ${unansweredCount === 1 ? "question" : "questions"} unanswered`}
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
                        <strong>
                          No {assessmentType.toUpperCase()} categories are
                          available.
                        </strong>
                        <span>Check the API question configuration.</span>
                      </div>
                    )}
                    {launchError && (
                      <div className="inline-error" role="alert">
                        {launchError}
                      </div>
                    )}
                    {managerCanValidate && activeLaunch && (
                      <div className="manager-approval">
                        <div>
                          <div className="eyebrow">TIER VALIDATION</div>
                          <strong>
                            Calculated suggestion:{" "}
                            {activeLaunch.tierId
                              ? tierById.get(activeLaunch.tierId)?.name ??
                                "Tier calculated"
                              : "Not available"}
                          </strong>
                          <p>
                            Accept the calculated tier, or choose an override
                            after reviewing the answers and applicable requirements.
                          </p>
                        </div>
                        <label className="manager-tier-select">
                          <span>Tier decision</span>
                          <select
                            aria-label="Tier decision"
                            value={tierOverride}
                            onChange={(event) =>
                              setTierOverride(event.target.value)
                            }
                          >
                            <option value="">Accept calculated tier</option>
                            {tiers.map((tier) => (
                              <option key={tier.id} value={tier.id}>
                                {tier.name} · {tier.title}
                              </option>
                            ))}
                          </select>
                        </label>
                        <button
                          className="button button-primary"
                          onClick={() => void validateLaunchTier()}
                          disabled={launchSaving}
                        >
                          {launchSaving ? "Validating…" : "Validate tier"}
                        </button>
                      </div>
                    )}
                    {assessmentType === "shield" &&
                      !assessmentReadOnly &&
                      activeShield && (
                        <div className="launch-actions">
                          <button
                            className="button button-secondary"
                            onClick={() =>
                              activeShield.state === "waiting-4-evidences"
                                ? void saveShieldEvidences(false)
                                : void saveShieldAnswers(false)
                            }
                            disabled={shieldSaving || questions.length === 0}
                          >
                            {shieldSaving ? "Saving…" : "Save progress"}
                          </button>
                          <button
                            className="button button-primary"
                            onClick={() =>
                              activeShield.state === "waiting-4-evidences"
                                ? void saveShieldEvidences(true)
                                : void saveShieldAnswers(true)
                            }
                            disabled={shieldSaving || questions.length === 0}
                          >
                            {shieldSaving
                              ? "Submitting…"
                              : activeShield.state === "waiting-4-evidences"
                                ? "Complete SHIELD"
                                : "Continue to evidence"}
                          </button>
                        </div>
                      )}
                    {!assessmentReadOnly &&
                      assessmentType === "launch" &&
                      !managerCanValidate && (
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
                    )}
                    {activeShield?.state === "evidences-submitted" && (
                      <p className="privacy-note">
                        Evidence was submitted. The workflow is finalizing the
                        exposure score.
                      </p>
                    )}
                    <p className="privacy-note">
                      {assessmentReadOnly
                        ? "This saved assessment is read-only. Use Back to asset details to return to the assessment history."
                        : assessmentType === "shield"
                          ? activeShield?.state === "waiting-4-evidences"
                            ? "Provide a description, a source link, or both for every required question. You can save partial evidence before completing SHIELD."
                            : "Rate observable conditions only. You can save partial answers before continuing to evidence."
                          : "Your answers are stored by the PSAP API. You may save partial progress and return later."}
                    </p>
                  </div>
                </div>
              </>
            ) : (
              <div className="empty-state">
                <strong>No assessment selected.</strong>
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
          ownerId={activeWorkerId}
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
