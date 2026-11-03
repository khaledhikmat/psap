import { useMemo, useState } from "react";
import {
  assetStateLabel,
  formatDate,
  launchStateLabel,
  locationLabel,
  shieldStateLabel,
} from "./format";
import type { Asset, Launch, Shield, Tier, Worker } from "./types";

type Tab = "assets" | "launches" | "shields";

type Props = {
  assets: Asset[];
  launches: Launch[];
  shields: Shield[];
  workers: Worker[];
  tiers: Tier[];
  onOpenAsset: (asset: Asset) => void;
};

const launchOpen = new Set([
  "waiting-4-answers",
  "answers-submitted",
  "waiting-4-tier-approval",
]);
const shieldOpen = new Set([
  "waiting-4-answers",
  "answers-submitted",
  "waiting-4-evidences",
  "evidences-submitted",
]);

function matches(haystack: string[], query: string): boolean {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
  const text = haystack.join(" ").toLowerCase();
  return terms.every((term) => text.includes(term));
}

export function AdminPortal({
  assets,
  launches,
  shields,
  workers,
  tiers,
  onOpenAsset,
}: Props) {
  const [tab, setTab] = useState<Tab>("assets");
  const [search, setSearch] = useState("");

  const assetById = useMemo(
    () => new Map(assets.map((asset) => [asset.id, asset])),
    [assets],
  );
  const workerName = (id?: string) =>
    (id && workers.find((worker) => worker.id === id)?.name) || "—";
  const tierName = (id?: string) =>
    (id && tiers.find((tier) => tier.id === id)?.name) || "—";

  // One searchable record per row so every column can be matched.
  const assetRows = useMemo(
    () =>
      [...assets]
        .sort((a, b) => a.name.localeCompare(b.name))
        .map((asset) => ({
          asset,
          owner: workerName(asset.businessOwner),
          manager: workerName(asset.businessManager),
          tier: tierName(asset.tierId),
        }))
        .filter((row) =>
          matches(
            [
              row.asset.name,
              row.asset.id,
              locationLabel(row.asset),
              row.owner,
              row.manager,
              row.tier,
              assetStateLabel(row.asset.state),
            ],
            search,
          ),
        ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [assets, workers, tiers, search],
  );

  const launchRows = useMemo(
    () =>
      [...launches]
        .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
        .map((launch) => {
          const asset = assetById.get(launch.assetId);
          return {
            launch,
            asset,
            owner: workerName(asset?.businessOwner),
            manager: workerName(asset?.businessManager),
          };
        })
        .filter((row) =>
          matches(
            [
              row.asset?.name ?? row.launch.assetId,
              row.asset ? locationLabel(row.asset) : "",
              row.owner,
              row.manager,
              tierName(row.launch.tierId),
              launchStateLabel(row.launch.state),
            ],
            search,
          ),
        ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [launches, assetById, workers, tiers, search],
  );

  const shieldRows = useMemo(
    () =>
      [...shields]
        .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
        .map((shield) => {
          const asset = assetById.get(shield.assetId);
          return {
            shield,
            asset,
            owner: workerName(asset?.businessOwner),
            manager: workerName(asset?.businessManager),
          };
        })
        .filter((row) =>
          matches(
            [
              row.asset?.name ?? row.shield.assetId,
              row.asset ? locationLabel(row.asset) : "",
              row.owner,
              row.manager,
              shieldStateLabel(row.shield.state),
            ],
            search,
          ),
        ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [shields, assetById, workers, search],
  );

  const total = { assets: assets.length, launches: launches.length, shields: shields.length };
  const shown = {
    assets: assetRows.length,
    launches: launchRows.length,
    shields: shieldRows.length,
  };
  const openLaunches = launches.filter((l) => launchOpen.has(l.state)).length;
  const openShields = shields.filter((s) => shieldOpen.has(s.state)).length;
  const tiered = assets.filter((asset) => asset.tierId).length;

  return (
    <section aria-labelledby="dashboard-title">
      <div className="page-heading">
        <div className="eyebrow">ADMIN PORTAL</div>
        <h1 id="dashboard-title">All assets, LAUNCHes and SHIELDs</h1>
        <p>
          A read-only view across every asset. Search by location, business
          owner, or business manager.
        </p>
      </div>

      <div className="metric-grid">
        <div className="metric-card metric-blue">
          <div className="metric-label">Assets</div>
          <div className="metric-value">{assets.length}</div>
          <div className="metric-caption">{tiered} with a validated tier</div>
        </div>
        <div className="metric-card metric-amber">
          <div className="metric-label">LAUNCHes</div>
          <div className="metric-value">{launches.length}</div>
          <div className="metric-caption">{openLaunches} in progress</div>
        </div>
        <div className="metric-card metric-navy">
          <div className="metric-label">SHIELDs</div>
          <div className="metric-value">{shields.length}</div>
          <div className="metric-caption">{openShields} in progress</div>
        </div>
      </div>

      <div className="admin-tabs" role="tablist" aria-label="Admin lists">
        {(["assets", "launches", "shields"] as Tab[]).map((key) => (
          <button
            key={key}
            role="tab"
            aria-selected={tab === key}
            className={`admin-tab${tab === key ? " admin-tab-active" : ""}`}
            onClick={() => setTab(key)}
          >
            {key === "assets"
              ? "Assets"
              : key === "launches"
                ? "LAUNCHes"
                : "SHIELDs"}{" "}
            <span className="count-badge">{total[key]}</span>
          </button>
        ))}
      </div>

      <div className="asset-toolbar">
        <label className="asset-search">
          <span>Search</span>
          <input
            type="search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Asset, location, business owner, business manager, status"
          />
        </label>
        <span className="muted" aria-live="polite">
          Showing {shown[tab]} of {total[tab]}
        </span>
      </div>

      {shown[tab] === 0 ? (
        <div className="panel asset-search-empty">
          <strong>No matching records</strong>
          <span>Try a different location, owner, manager, or status.</span>
        </div>
      ) : (
        <div className="asset-table-wrap">
          {tab === "assets" && (
            <table className="asset-table" aria-label="All assets">
              <thead>
                <tr>
                  <th scope="col">Asset</th>
                  <th scope="col">Location</th>
                  <th scope="col">Business owner</th>
                  <th scope="col">Business manager</th>
                  <th scope="col">Tier</th>
                  <th scope="col">Status</th>
                </tr>
              </thead>
              <tbody>
                {assetRows.map((row) => (
                  <tr key={row.asset.id}>
                    <th scope="row">
                      <button
                        className="asset-table-name"
                        onClick={() => onOpenAsset(row.asset)}
                      >
                        {row.asset.name}
                      </button>
                      <span className="asset-table-id">{row.asset.id}</span>
                    </th>
                    <td>{locationLabel(row.asset) || "—"}</td>
                    <td>{row.owner}</td>
                    <td>{row.manager}</td>
                    <td>{row.tier}</td>
                    <td>{assetStateLabel(row.asset.state)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {tab === "launches" && (
            <table className="asset-table" aria-label="All LAUNCHes, newest first">
              <thead>
                <tr>
                  <th scope="col">Asset</th>
                  <th scope="col">Location</th>
                  <th scope="col">Business owner</th>
                  <th scope="col">Business manager</th>
                  <th scope="col">Tier</th>
                  <th scope="col">Status</th>
                  <th scope="col">Last updated</th>
                </tr>
              </thead>
              <tbody>
                {launchRows.map((row) => (
                  <tr key={row.launch.id}>
                    <th scope="row">
                      {row.asset ? (
                        <button
                          className="asset-table-name"
                          onClick={() => onOpenAsset(row.asset!)}
                        >
                          {row.asset.name}
                        </button>
                      ) : (
                        row.launch.assetId
                      )}
                    </th>
                    <td>{row.asset ? locationLabel(row.asset) || "—" : "—"}</td>
                    <td>{row.owner}</td>
                    <td>{row.manager}</td>
                    <td>{tierName(row.launch.tierId)}</td>
                    <td>{launchStateLabel(row.launch.state)}</td>
                    <td>{formatDate(row.launch.updatedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {tab === "shields" && (
            <table className="asset-table" aria-label="All SHIELDs, newest first">
              <thead>
                <tr>
                  <th scope="col">Asset</th>
                  <th scope="col">Location</th>
                  <th scope="col">Business owner</th>
                  <th scope="col">Business manager</th>
                  <th scope="col">Score</th>
                  <th scope="col">Status</th>
                  <th scope="col">Last updated</th>
                </tr>
              </thead>
              <tbody>
                {shieldRows.map((row) => (
                  <tr key={row.shield.id}>
                    <th scope="row">
                      {row.asset ? (
                        <button
                          className="asset-table-name"
                          onClick={() => onOpenAsset(row.asset!)}
                        >
                          {row.asset.name}
                        </button>
                      ) : (
                        row.shield.assetId
                      )}
                    </th>
                    <td>{row.asset ? locationLabel(row.asset) || "—" : "—"}</td>
                    <td>{row.owner}</td>
                    <td>{row.manager}</td>
                    <td>{row.shield.score ?? "—"}</td>
                    <td>{shieldStateLabel(row.shield.state)}</td>
                    <td>{formatDate(row.shield.updatedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </section>
  );
}
