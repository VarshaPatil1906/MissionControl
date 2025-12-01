import React, { useState } from "react";

const API_BASE = "http://localhost:8080";

function MissionHistoryViewer() {
  const [missionId, setMissionId] = useState("");
  const [events, setEvents] = useState([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

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
      const res = await fetch(`${API_BASE}/missions/${missionId}/history`);
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
    <div
      style={{
        padding: "1rem",
        borderRadius: "8px",
        background: "#f5f5f5",
        marginTop: "1rem",
      }}
    >
      <h3>Lookup Mission History</h3>
      <div
        style={{ display: "flex", gap: "0.5rem", marginBottom: "0.5rem" }}
      >
        <input
          type="text"
          placeholder="Enter Mission ID"
          value={missionId}
          onChange={(e) => setMissionId(e.target.value)}
          style={{ flex: 1, padding: "0.4rem" }}
        />
        <button onClick={fetchHistory} style={{ padding: "0.4rem 0.8rem" }}>
          Fetch
        </button>
      </div>

      {loading && <p>Loading...</p>}
      {error && <p style={{ color: "red" }}>{error}</p>}

      {events.length > 0 && (
        <ul style={{ listStyle: "none", paddingLeft: 0 }}>
          {events.map((ev, idx) => (
            <li key={idx} style={{ marginBottom: "0.3rem" }}>
              <strong>{ev.status}</strong>
              {ev.soldier && <> by {ev.soldier}</>}
              {" "}
              at {new Date(ev.time).toLocaleString()}
              {ev.message && ` – ${ev.message}`}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export default MissionHistoryViewer;
