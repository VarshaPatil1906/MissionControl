import React, { useState, useEffect } from "react";
import "./App.css";

function MissionCard({ mission, index }) {
  let statusColor;
  if (mission.status === "COMPLETED") statusColor = "green";
  else if (mission.status === "FAILED") statusColor = "red";
  else if (mission.status === "IN_PROGRESS") statusColor = "orange";
  else statusColor = "#7b7b7b";

  return (
    <div className="mission-card">
      <div>
        <span className="status-label" style={{ background: statusColor }}>
          {mission.status}
        </span>
        <span className="mission-id">{`Mission #${index + 1} • ${mission.mission_id.slice(0, 8)}...`}</span>
      </div>
      <div className="mission-payload">{mission.payload}</div>
    </div>
  );
}

function App() {
  const [missions, setMissions] = useState([]);
  const [desc, setDesc] = useState("");

  const fetchMissions = () => {
    fetch("http://localhost:8080/missions")
      .then((res) => res.json())
      .then((data) => setMissions(data.reverse()))
      .catch(() => setMissions([]));
  };

  useEffect(() => {
    fetchMissions();
    const interval = setInterval(fetchMissions, 5000);
    return () => clearInterval(interval);
  }, []);

  const submitMission = () => {
    if (!desc.trim()) return;
    fetch("http://localhost:8080/missions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ payload: desc }),
    }).then(() => {
      setDesc("");
      setTimeout(fetchMissions, 1000);
    });
  };

  const queued = missions.filter((m) => m.status === "QUEUED").length;
  const inProgress = missions.filter((m) => m.status === "IN_PROGRESS").length;
  const completed = missions.filter((m) => m.status === "COMPLETED").length;
  const failed = missions.filter((m) => m.status === "FAILED").length;

  return (
    <div className="dashboard-container">
      <header>
        <h1>Mission Control</h1>
      </header>
      <div className="main-grid">
        <section className="new-mission-panel">
          <h2 className="panel-header">Create New Mission</h2>
          <label className="panel-label">Mission Description</label>
          <textarea
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            placeholder="Additional mission details and instructions..."
            className="desc-box"
          />
          <button className="action-btn" onClick={submitMission}>
            Deploy Mission
          </button>
        </section>
        <section className="status-panel">
          <h2 className="panel-header">Mission Status</h2>
          <div className="status-stats">
            <div style={{ color: "#7b7b7b" }}>Queued: <b>{queued}</b></div>
            <div style={{ color: "orange" }}>In Progress: <b>{inProgress}</b></div>
            <div style={{ color: "green" }}>Completed: <b>{completed}</b></div>
            <div style={{ color: "red" }}>Failed: <b>{failed}</b></div>
          </div>
          <button className="action-btn" onClick={fetchMissions}>
            Refresh
          </button>
          <div className="mission-list">
            {missions.length === 0 ? (
              <p>No missions found.</p>
            ) : (
              missions.map((mission, i) => (
                <MissionCard key={mission.mission_id} mission={mission} index={i} />
              ))
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

export default App;
