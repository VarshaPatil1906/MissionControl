import React, { useState, useEffect } from "react";
import "./App.css";

const commanderPorts = {
  commander1: 8081,
  commander2: 8082,
  commander3: 8083,
};

function MissionCard({ mission, index, onSelect, commander }) {
  let statusColor;
  if (mission.status === "COMPLETED") statusColor = "green";
  else if (mission.status === "FAILED") statusColor = "red";
  else if (mission.status === "IN_PROGRESS") statusColor = "orange";
  else statusColor = "#7b7b7b";

  return (
    <div
      className="mission-card"
      onClick={() => onSelect(mission.mission_id)}
      style={{ cursor: "pointer" }}
    >
      <div>
        <span className="status-label" style={{ background: statusColor }}>
          {mission.status}
        </span>
        <span className="mission-id">
          {`Mission #${index + 1} • ${mission.mission_id.slice(0, 8)}...`}
        </span>
      </div>
      <div className="mission-payload">
        {mission.payload}
        <span
          style={{
            marginLeft: "8px",
            fontSize: "0.85rem",
            color: "#666",
          }}
        >
          (Commander: {commander}
          {mission.target_soldier && ` → Soldier: ${mission.target_soldier}`})
        </span>
      </div>
      <div
        style={{
          fontSize: "0.75rem",
          color: "#666",
          marginTop: "0.25rem",
          wordBreak: "break-all",
        }}
      >
        ID: {mission.mission_id}
      </div>
    </div>
  );
}

function MissionHistoryViewer({ selectedMissionId, apiBase, commander }) {
  const [missionId, setMissionId] = useState("");
  const [events, setEvents] = useState([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (selectedMissionId) {
      setMissionId(selectedMissionId);
      setEvents([]);
      setError("");
    }
  }, [selectedMissionId]);

  const fetchHistory = async () => {
    if (!missionId.trim()) {
      setError("Please enter a mission ID");
      setEvents([]);
      return;
    }

    setLoading(true);
    setError("");
    setEvents([]);

    try {
      const res = await fetch(`${apiBase}/missions/${missionId}/history`);
      if (!res.ok) {
        setError("Mission not found or no history available");
        setLoading(false);
        return;
      }
      const data = await res.json();
      setEvents(data.events || []);
    } catch (e) {
      setError("Failed to load mission history");
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="history-panel">
      <h2 className="panel-header">Mission History Lookup</h2>
      <div className="history-input-row">
        <input
          type="text"
          placeholder="Enter Mission ID"
          value={missionId}
          onChange={(e) => setMissionId(e.target.value)}
          className="history-input wide-input"
        />
        <button className="action-btn" onClick={fetchHistory}>
          Fetch History
        </button>
      </div>
      {loading && <p>Loading...</p>}
      {error && <p style={{ color: "red" }}>{error}</p>}
      {events.length > 0 && (
        <ul className="history-list">
          {events.map((ev, idx) => (
            <li key={idx} className="history-item">
              <strong>{ev.status}</strong>
              {ev.soldier && <> by {ev.soldier}</>}
              {" "}
              (Commander: {commander})
              {" "}
              at {new Date(ev.time).toLocaleString()}
              {ev.message && ` – ${ev.message}`}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function App() {
  const [selectedCommander, setSelectedCommander] = useState("commander1");
  const API_BASE = `http://localhost:${commanderPorts[selectedCommander]}`;

  const [missions, setMissions] = useState([]);
  const [desc, setDesc] = useState("");
  const [targetSoldier, setTargetSoldier] = useState("soldier1");
  const [selectedMissionId, setSelectedMissionId] = useState("");

  const [tokenSoldier, setTokenSoldier] = useState("soldier1");
  const [currentToken, setCurrentToken] = useState("");
  const [tokenError, setTokenError] = useState("");

  useEffect(() => {
    fetch(`${API_BASE}/missions`)
      .then((res) => res.json())
      .then((data) => {
        setMissions(data.slice().reverse());
      })
      .catch(() => setMissions([]));
  }, [API_BASE]);

  const fetchMissions = () => {
    fetch(`${API_BASE}/missions`)
      .then((res) => res.json())
      .then((data) => {
        const incoming = data.slice().reverse();
        setMissions((prev) => {
          const byId = new Map(prev.map((m) => [m.mission_id, m]));
          const result = [...prev];
          incoming.forEach((m) => {
            if (!byId.has(m.mission_id)) {
              result.unshift(m);
            } else {
              const idx = result.findIndex(
                (x) => x.mission_id === m.mission_id
              );
              if (idx !== -1) result[idx] = { ...result[idx], ...m };
            }
          });
          return result;
        });
      })
      .catch(() => {});
  };

  const submitMission = () => {
    if (!desc.trim()) return;
    fetch(`${API_BASE}/missions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        payload: desc,
        target_soldier: targetSoldier,
      }),
    }).then(() => {
      setDesc("");
      setTimeout(fetchMissions, 1000);
    });
  };

  const queued = missions.filter((m) => m.status === "QUEUED").length;
  const inProgress = missions.filter((m) => m.status === "IN_PROGRESS").length;
  const completed = missions.filter((m) => m.status === "COMPLETED").length;
  const failed = missions.filter((m) => m.status === "FAILED").length;

  const fetchSoldierToken = async () => {
    setTokenError("");
    setCurrentToken("");
    try {
      const res = await fetch(
        `${API_BASE}/soldiers/${tokenSoldier}/token`
      );
      if (!res.ok) {
        setTokenError("No token for this soldier yet");
        return;
      }
      const data = await res.json();
      setCurrentToken(data.token || "");
    } catch (e) {
      setTokenError("Failed to load token");
    }
  };

  useEffect(() => {
    const id = setInterval(() => {
      fetchSoldierToken();
    }, 5000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tokenSoldier, API_BASE]);

  return (
    <div className="dashboard-container">
      <header>
        <h1>Mission Control</h1>
      </header>

      <div className="main-grid">
        <section className="new-mission-panel">
          <h2 className="panel-header">Create New Mission</h2>

          <label className="panel-label">Commander</label>
          <select
            value={selectedCommander}
            onChange={(e) => setSelectedCommander(e.target.value)}
            className="commander-select"
          >
            <option value="commander1">Commander 1</option>
            <option value="commander2">Commander 2</option>
            <option value="commander3">Commander 3</option>
          </select>

          <label className="panel-label">Mission Description</label>
          <textarea
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            placeholder="Additional mission details and instructions..."
            className="desc-box"
          />

          <label className="panel-label">Target Soldier</label>
          <select
            value={targetSoldier}
            onChange={(e) => setTargetSoldier(e.target.value)}
            className="soldier-select"
          >
            <option value="soldier1">Soldier 1</option>
            <option value="soldier2">Soldier 2</option>
            <option value="soldier3">Soldier 3</option>
          </select>

          <button className="action-btn" onClick={submitMission}>
            Deploy Mission
          </button>
        </section>

        <section className="status-panel">
          <h2 className="panel-header">Mission Status</h2>
          <div className="status-stats">
            <div style={{ color: "#7b7b7b" }}>
              Queued: <b>{queued}</b>
            </div>
            <div style={{ color: "orange" }}>
              In Progress: <b>{inProgress}</b>
            </div>
            <div style={{ color: "green" }}>
              Completed: <b>{completed}</b>
            </div>
            <div style={{ color: "red" }}>
              Failed: <b>{failed}</b>
            </div>
          </div>
          <button className="action-btn" onClick={fetchMissions}>
            Refresh
          </button>
          <div className="mission-list">
            {missions.length === 0 ? (
              <p>No missions found.</p>
            ) : (
              missions.map((mission, i) => (
                <MissionCard
                  key={mission.mission_id}
                  mission={mission}
                  index={i}
                  onSelect={setSelectedMissionId}
                  commander={selectedCommander}
                />
              ))
            )}
          </div>
        </section>
      </div>

      <div className="bottom-grid">
        <section className="history-panel half-panel">
          <h2 className="panel-header">Soldier Token Viewer</h2>
          <div className="history-input-row">
            <select
              value={tokenSoldier}
              onChange={(e) => setTokenSoldier(e.target.value)}
              className="soldier-select wide-input"
            >
              <option value="soldier1">Soldier 1</option>
              <option value="soldier2">Soldier 2</option>
              <option value="soldier3">Soldier 3</option>
            </select>
            <button className="action-btn" onClick={fetchSoldierToken}>
              Fetch Token
            </button>
          </div>
          {tokenError && <p style={{ color: "red" }}>{tokenError}</p>}
          {currentToken && (
            <p style={{ wordBreak: "break-all", marginTop: "0.5rem" }}>
              Current token for {tokenSoldier}: {currentToken}
            </p>
          )}
        </section>

        <section className="half-panel">
          <MissionHistoryViewer
            selectedMissionId={selectedMissionId}
            apiBase={API_BASE}
            commander={selectedCommander}
          />
        </section>
      </div>
    </div>
  );
}

export default App;
