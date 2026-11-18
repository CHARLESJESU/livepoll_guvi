import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";

const configuredAPI = import.meta.env.VITE_API_URL;
if (import.meta.env.PROD && !configuredAPI) throw new Error("VITE_API_URL is required for production builds");
const API = configuredAPI || "http://localhost:8080";
const getToken = () => localStorage.getItem("livepoll-token") || "";
const voterId = () => {
  let id = localStorage.getItem("livepoll-voter");
  if (!id) { id = crypto.randomUUID(); localStorage.setItem("livepoll-voter", id); }
  return id;
};
async function request(path, options = {}) {
  const res = await fetch(API + path, { ...options, headers: { "Content-Type": "application/json", ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}), ...options.headers } });
  let body = {};
  try { body = await res.json(); } catch { /* Proxy/network failures may not return JSON. */ }
  if (!res.ok) throw new Error(body.error || "Something went wrong");
  return body;
}

function Auth({ onDone }) {
  const [signup, setSignup] = useState(false), [email, setEmail] = useState(""), [password, setPassword] = useState(""), [error, setError] = useState(""), [submitting, setSubmitting] = useState(false);
  async function submit(e) {
    e.preventDefault(); setError(""); setSubmitting(true);
    try { const data = await request(`/api/auth/${signup ? "signup" : "login"}`, { method: "POST", body: JSON.stringify({ email, password }) }); localStorage.setItem("livepoll-token", data.token); onDone(); }
    catch (err) { setError(err.message); }
    finally { setSubmitting(false); }
  }
  return <section className="card auth"><p className="eyebrow">Creator access</p><h2>{signup ? "Create your account" : "Welcome back"}</h2><form onSubmit={submit}><input required type="email" placeholder="you@example.com" value={email} onChange={e => setEmail(e.target.value)} /><input required minLength="8" maxLength="72" type="password" placeholder="Password (8+ characters)" value={password} onChange={e => setPassword(e.target.value)} /><button disabled={submitting}>{submitting ? "Please wait…" : signup ? "Sign up" : "Sign in"}</button></form>{error && <p className="error">{error}</p>}<button className="link" onClick={() => setSignup(!signup)}>{signup ? "Already have an account? Sign in" : "New here? Create an account"}</button></section>;
}

function CreatePoll({ onCreated }) {
  const [question, setQuestion] = useState("What should we build next?"), [options, setOptions] = useState(["A mobile app", "A browser extension"]), [error, setError] = useState(""), [submitting, setSubmitting] = useState(false);
  async function submit(e) {
    e.preventDefault(); setError(""); setSubmitting(true);
    try { const data = await request("/api/polls", { method: "POST", body: JSON.stringify({ question, options }) }); onCreated(data.poll.id); }
    catch (err) { setError(err.message); }
    finally { setSubmitting(false); }
  }
  return <section className="card create"><p className="eyebrow">Make a room</p><h1>Ask. Share. See it happen.</h1><p className="muted">Create a question and watch the room move in real time.</p><form onSubmit={submit}><label>Question<input required minLength="5" maxLength="200" value={question} onChange={e => setQuestion(e.target.value)} /></label><label>Answer options</label>{options.map((option, i) => <div className="option-row" key={i}><input required maxLength="80" value={option} onChange={e => setOptions(options.map((v, j) => j === i ? e.target.value : v))} /><button type="button" className="remove" disabled={options.length <= 2} onClick={() => setOptions(options.filter((_, j) => i !== j))}>×</button></div>)}{options.length < 6 && <button type="button" className="link add" onClick={() => setOptions([...options, ""])}>+ Add option</button>}<button disabled={submitting}>{submitting ? "Creating…" : "Create poll →"}</button></form>{error && <p className="error">{error}</p>}</section>;
}

function Poll({ id }) {
  const [data, setData] = useState(null), [error, setError] = useState(""), [voted, setVoted] = useState(false), [submitting, setSubmitting] = useState(false), [streamState, setStreamState] = useState("connecting"), [copied, setCopied] = useState(false);
  useEffect(() => {
    request(`/api/polls/${id}`).then(setData).catch(e => setError(e.message));
    const source = new EventSource(`${API}/api/polls/${id}/events`);
    source.onopen = () => setStreamState("live");
    source.onmessage = e => setData(prev => prev ? { ...prev, counts: JSON.parse(e.data) } : prev);
    source.onerror = () => setStreamState("reconnecting");
    return () => source.close();
  }, [id]);
  async function vote(option) {
    setError(""); setSubmitting(true);
    try {
      const result = await request(`/api/polls/${id}/votes`, { method: "POST", body: JSON.stringify({ option, voter: voterId() }) });
      setData(prev => ({ ...prev, counts: result.counts }));
      setVoted(true);
    } catch (e) {
      setError(e.message);
      if (e.message === "you have already voted") setVoted(true);
    } finally { setSubmitting(false); }
  }
  async function copyLink() {
    try { await navigator.clipboard.writeText(location.href); setCopied(true); setTimeout(() => setCopied(false), 1800); }
    catch { setError("Copy failed. Use the address bar to copy the poll link."); }
  }
  if (error && !data) return <section className="card poll-error"><p className="eyebrow">Poll unavailable</p><h1>We couldn't find that poll.</h1><p className="error">{error}</p><button onClick={() => { history.replaceState({}, "", "/"); location.reload(); }}>Back to Pulse</button></section>;
  if (!data) return <section className="card"><p className="eyebrow">Loading poll</p><p className="muted">Fetching poll and live results…</p></section>;
  const { poll, counts } = data, max = Math.max(...counts.options, 1);
  return <section className="card poll"><p className="eyebrow">Live poll <span className={`live ${streamState}`}>● {streamState === "live" ? "LIVE" : streamState === "reconnecting" ? "RECONNECTING" : "CONNECTING"}</span></p><h1>{poll.question}</h1><p className="muted">{counts.total} {counts.total === 1 ? "vote" : "votes"} · Updates automatically</p>{poll.options.map((option, i) => <button className={`result ${voted ? "locked" : ""}`} disabled={voted || submitting} onClick={() => vote(i)} key={option}><span className="result-label">{option}<strong>{counts.options[i]}</strong></span><span className="bar" style={{ width: `${counts.options[i] / max * 100}%` }} /></button>)}{voted ? <p className="success">Vote recorded. Watch the results change live.</p> : <p className="hint">{submitting ? "Recording your vote…" : "Choose an answer to vote"}</p>}{error && <p className="error">{error}</p>}<button className="link" onClick={copyLink}>{copied ? "Link copied!" : "Copy poll link"}</button></section>;
}

function App() {
  const [id, setId] = useState(new URLSearchParams(location.search).get("poll")), [auth, setAuth] = useState(!!getToken());
  function openPoll(pollId) { history.replaceState({}, "", `?poll=${pollId}`); setId(pollId); }
  const view = useMemo(() => id ? <Poll id={id} /> : auth ? <CreatePoll onCreated={openPoll} /> : <Auth onDone={() => setAuth(true)} />, [id, auth]);
  return <main><header><a className="brand" href="/">PULSE<span>.</span></a>{auth && <button className="signout" onClick={() => { localStorage.removeItem("livepoll-token"); setAuth(false); }}>Sign out</button>}</header>{view}<footer>Built for conversations that move.</footer></main>;
}
createRoot(document.getElementById("root")).render(<App />);
