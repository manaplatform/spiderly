package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"spiderly/internal/models"
)

// Server manages the web dashboard
type Server struct {
	port   int
	hub    *Hub
	server *http.Server
}

// NewServer creates a new web server
func NewServer(port int) *Server {
	if port == 0 {
		port = 8080
	}

	s := &Server{
		port: port,
		hub:  NewHub(),
	}

	return s
}

// Start begins serving the web dashboard
func (s *Server) Start() error {
	go s.hub.Run()

	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/stats", s.handleStats)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Broadcast sends a message to all connected clients
func (s *Server) Broadcast(msg models.WebSocketMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal broadcast message: %v", err)
		return
	}
	s.hub.Broadcast(data)
}

// handleWebSocket upgrades HTTP to WebSocket
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.hub.HandleWebSocket(w, r)
}

// handleStats returns current stats as JSON
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "running"})
}

// handleDashboard serves the main dashboard HTML
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="fa" dir="rtl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🕷️ Spiderly — داشبورد خزنده هوشمند</title>
    <style>
        /* ───────────────────────── CSS VARIABLES ───────────────────────── */
        :root {
            --bg-primary:    #0a1a0f;
            --bg-secondary:  #0f2818;
            --bg-tertiary:   #163822;
            --bg-card:       #122e1c;
            --bg-card-hover: #1a4028;
            --bg-input:      #0d200f;

            --green-50:  #f0fdf4;
            --green-100: #dcfce7;
            --green-200: #bbf7d0;
            --green-300: #86efac;
            --green-400: #4ade80;
            --green-500: #22c55e;
            --green-600: #16a34a;
            --green-700: #15803d;
            --green-800: #166534;
            --green-900: #14532d;

            --accent:       #4ade80;
            --accent-dim:   #22c55e;
            --accent-glow:  rgba(74, 222, 128, 0.15);
            --accent-glow2: rgba(74, 222, 128, 0.08);

            --text-primary:   #e2e8f0;
            --text-secondary: #94a3b8;
            --text-muted:     #64748b;
            --text-bright:    #f0fdf4;

            --success:  #4ade80;
            --warning:  #fbbf24;
            --error:    #f87171;
            --info:     #38bdf8;
            --sitemap:  #a78bfa;

            --border:       rgba(74, 222, 128, 0.12);
            --border-light: rgba(74, 222, 128, 0.06);
            --shadow:       0 4px 24px rgba(0, 0, 0, 0.4);
            --shadow-lg:    0 8px 40px rgba(0, 0, 0, 0.5);

            --radius:    0.75rem;
            --radius-lg: 1rem;
            --radius-xl: 1.25rem;

            --font-sans: 'Vazirmatn', 'Segoe UI', system-ui, -apple-system, sans-serif;
            --font-mono: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
        }

        /* ───────────────────────── RESET ───────────────────────── */
        *, *::before, *::after {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        html {
            scroll-behavior: smooth;
        }

        body {
            font-family: var(--font-sans);
            background: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            line-height: 1.7;
            direction: rtl;
            overflow-x: hidden;
        }

        /* Scrollbar */
        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-track { background: var(--bg-primary); }
        ::-webkit-scrollbar-thumb { background: var(--green-800); border-radius: 3px; }
        ::-webkit-scrollbar-thumb:hover { background: var(--green-700); }

        /* ───────────────────────── HEADER ───────────────────────── */
        .header {
            background: linear-gradient(135deg, var(--bg-secondary) 0%, var(--bg-tertiary) 50%, var(--bg-secondary) 100%);
            border-bottom: 1px solid var(--border);
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            position: sticky;
            top: 0;
            z-index: 100;
            backdrop-filter: blur(20px);
        }

        .header-right {
            display: flex;
            align-items: center;
            gap: 1rem;
        }

        .logo {
            display: flex;
            align-items: center;
            gap: 0.75rem;
        }

        .logo-icon {
            width: 42px;
            height: 42px;
            background: linear-gradient(135deg, var(--green-500), var(--green-700));
            border-radius: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 1.4rem;
            box-shadow: 0 0 20px rgba(74, 222, 128, 0.3);
        }

        .logo-text h1 {
            font-size: 1.25rem;
            font-weight: 800;
            color: var(--green-300);
            letter-spacing: -0.02em;
        }

        .logo-text span {
            font-size: 0.7rem;
            color: var(--text-muted);
            font-weight: 400;
        }

        .header-left {
            display: flex;
            align-items: center;
            gap: 1rem;
        }

        .target-url {
            background: var(--bg-input);
            border: 1px solid var(--border);
            border-radius: var(--radius);
            padding: 0.4rem 1rem;
            font-size: 0.8rem;
            color: var(--accent);
            font-family: var(--font-mono);
            direction: ltr;
            max-width: 300px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .status-badge {
            padding: 0.3rem 0.85rem;
            border-radius: 9999px;
            font-size: 0.8rem;
            font-weight: 600;
            display: flex;
            align-items: center;
            gap: 0.5rem;
            transition: all 0.3s ease;
        }

        .status-badge.connecting {
            background: rgba(251, 191, 36, 0.15);
            color: var(--warning);
            border: 1px solid rgba(251, 191, 36, 0.3);
        }
        .status-badge.connected {
            background: rgba(74, 222, 128, 0.15);
            color: var(--success);
            border: 1px solid rgba(74, 222, 128, 0.3);
        }
        .status-badge.disconnected {
            background: rgba(248, 113, 113, 0.15);
            color: var(--error);
            border: 1px solid rgba(248, 113, 113, 0.3);
        }

        .status-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: currentColor;
            animation: pulse 2s ease-in-out infinite;
        }

        @keyframes pulse {
            0%, 100% { opacity: 1; transform: scale(1); }
            50% { opacity: 0.4; transform: scale(0.85); }
        }

        /* ───────────────────────── CONTAINER ───────────────────────── */
        .container {
            max-width: 1500px;
            margin: 0 auto;
            padding: 1.5rem;
        }

        /* ───────────────────────── STATS GRID ───────────────────────── */
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(5, 1fr);
            gap: 1rem;
            margin-bottom: 1.5rem;
        }

        @media (max-width: 1024px) {
            .stats-grid { grid-template-columns: repeat(3, 1fr); }
        }
        @media (max-width: 640px) {
            .stats-grid { grid-template-columns: repeat(2, 1fr); }
        }

        .stat-card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: var(--radius-lg);
            padding: 1.25rem;
            position: relative;
            overflow: hidden;
            transition: all 0.3s ease;
        }

        .stat-card::before {
            content: '';
            position: absolute;
            top: 0;
            right: 0;
            width: 80px;
            height: 80px;
            border-radius: 50%;
            filter: blur(30px);
            opacity: 0.1;
            pointer-events: none;
        }

        .stat-card:hover {
            transform: translateY(-2px);
            box-shadow: var(--shadow);
            border-color: rgba(74, 222, 128, 0.25);
        }

        .stat-card .stat-icon {
            font-size: 1.5rem;
            margin-bottom: 0.5rem;
        }

        .stat-card .stat-value {
            font-size: 2rem;
            font-weight: 800;
            line-height: 1;
            font-family: var(--font-mono);
        }

        .stat-card .stat-label {
            color: var(--text-secondary);
            font-size: 0.8rem;
            margin-top: 0.35rem;
        }

        .stat-card .stat-sub {
            font-size: 0.7rem;
            color: var(--text-muted);
            margin-top: 0.25rem;
        }

        .stat-card.pages .stat-value { color: var(--success); }
        .stat-card.pages::before { background: var(--success); }

        .stat-card.sitemap .stat-value { color: var(--sitemap); }
        .stat-card.sitemap::before { background: var(--sitemap); }

        .stat-card.links .stat-value { color: var(--info); }
        .stat-card.links::before { background: var(--info); }

        .stat-card.errors .stat-value { color: var(--error); }
        .stat-card.errors::before { background: var(--error); }

        .stat-card.speed .stat-value { color: var(--warning); }
        .stat-card.speed::before { background: var(--warning); }

        /* ───────────────────────── PROGRESS BAR ───────────────────────── */
        .progress-section {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: var(--radius-lg);
            padding: 1rem 1.5rem;
            margin-bottom: 1.5rem;
            display: flex;
            align-items: center;
            gap: 1.5rem;
        }

        .progress-info {
            flex-shrink: 0;
            min-width: 140px;
        }

        .progress-info .label {
            font-size: 0.8rem;
            color: var(--text-secondary);
        }

        .progress-info .percent {
            font-size: 1.5rem;
            font-weight: 800;
            color: var(--accent);
            font-family: var(--font-mono);
        }

        .progress-bar-wrapper {
            flex: 1;
        }

        .progress-track {
            height: 10px;
            background: var(--bg-primary);
            border-radius: 5px;
            overflow: hidden;
            position: relative;
        }

        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, var(--green-700), var(--green-500), var(--green-400));
            border-radius: 5px;
            transition: width 0.5s ease;
            position: relative;
            width: 0%;
        }

        .progress-fill::after {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: linear-gradient(90deg, transparent, rgba(255,255,255,0.15), transparent);
            animation: shimmer 2s infinite;
        }

        @keyframes shimmer {
            0% { transform: translateX(-100%); }
            100% { transform: translateX(100%); }
        }

        .progress-status {
            flex-shrink: 0;
            font-size: 0.75rem;
            color: var(--text-muted);
            font-family: var(--font-mono);
            direction: ltr;
            text-align: left;
        }

        /* ───────────────────────── MAIN LAYOUT ───────────────────────── */
        .main-grid {
            display: grid;
            grid-template-columns: 1fr 380px;
            gap: 1.5rem;
        }

        @media (max-width: 1024px) {
            .main-grid { grid-template-columns: 1fr; }
        }

        /* ───────────────────────── NEWS CARDS ───────────────────────── */
        .panel {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: var(--radius-xl);
            overflow: hidden;
        }

        .panel-header {
            padding: 1rem 1.5rem;
            border-bottom: 1px solid var(--border);
            display: flex;
            justify-content: space-between;
            align-items: center;
            background: linear-gradient(135deg, var(--bg-tertiary), var(--bg-card));
        }

        .panel-header h2 {
            font-size: 1rem;
            font-weight: 700;
            color: var(--green-300);
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .panel-badge {
            background: rgba(74, 222, 128, 0.15);
            color: var(--accent);
            padding: 0.2rem 0.6rem;
            border-radius: 9999px;
            font-size: 0.7rem;
            font-weight: 700;
            font-family: var(--font-mono);
        }

        .panel-controls {
            display: flex;
            gap: 0.5rem;
        }

        .panel-btn {
            background: var(--bg-primary);
            border: 1px solid var(--border);
            border-radius: 0.5rem;
            color: var(--text-secondary);
            padding: 0.3rem 0.7rem;
            font-size: 0.75rem;
            cursor: pointer;
            transition: all 0.2s;
        }

        .panel-btn:hover {
            background: var(--bg-tertiary);
            color: var(--text-primary);
            border-color: var(--accent-dim);
        }

        .panel-btn.active {
            background: rgba(74, 222, 128, 0.15);
            color: var(--accent);
            border-color: var(--accent-dim);
        }

        .news-container {
            padding: 1rem;
            max-height: 700px;
            overflow-y: auto;
        }

        .news-card {
            background: var(--bg-primary);
            border: 1px solid var(--border-light);
            border-radius: var(--radius);
            padding: 1.25rem;
            margin-bottom: 0.75rem;
            transition: all 0.3s ease;
            cursor: default;
            position: relative;
        }

        .news-card:hover {
            border-color: var(--accent-dim);
            background: rgba(10, 26, 15, 0.8);
            box-shadow: 0 0 30px rgba(74, 222, 128, 0.05);
        }

        .news-card-top {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 0.75rem;
        }

        .news-card .news-status {
            display: inline-flex;
            align-items: center;
            gap: 0.3rem;
            font-size: 0.7rem;
            font-weight: 600;
            padding: 0.15rem 0.5rem;
            border-radius: 9999px;
            flex-shrink: 0;
        }

        .news-status.s200 {
            background: rgba(74, 222, 128, 0.15);
            color: var(--success);
        }
        .news-status.s301, .news-status.s302 {
            background: rgba(251, 191, 36, 0.15);
            color: var(--warning);
        }
        .news-status.s400, .news-status.s403, .news-status.s404, .news-status.s500 {
            background: rgba(248, 113, 113, 0.15);
            color: var(--error);
        }

        .news-card .news-title {
            font-size: 1rem;
            font-weight: 700;
            color: var(--text-bright);
            line-height: 1.5;
            margin-bottom: 0.5rem;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
            overflow: hidden;
        }

        .news-card .news-url {
            font-size: 0.75rem;
            font-family: var(--font-mono);
            color: var(--accent-dim);
            direction: ltr;
            text-align: right;
            margin-bottom: 0.75rem;
            display: block;
            text-decoration: none;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .news-card .news-url:hover {
            color: var(--accent);
            text-decoration: underline;
        }

        .news-card .news-description {
            font-size: 0.85rem;
            color: var(--text-secondary);
            line-height: 1.7;
            margin-bottom: 0.75rem;
            display: -webkit-box;
            -webkit-line-clamp: 3;
            -webkit-box-orient: vertical;
            overflow: hidden;
        }

        .news-meta {
            display: flex;
            flex-wrap: wrap;
            gap: 0.75rem;
            margin-bottom: 0.75rem;
        }

        .news-meta-item {
            display: flex;
            align-items: center;
            gap: 0.3rem;
            font-size: 0.72rem;
            color: var(--text-muted);
        }

        .news-meta-item .meta-icon {
            font-size: 0.85rem;
        }

        .news-meta-item .meta-value {
            color: var(--text-secondary);
        }

        .news-tags {
            display: flex;
            flex-wrap: wrap;
            gap: 0.4rem;
            margin-top: 0.5rem;
        }

        .news-tag {
            background: var(--bg-tertiary);
            color: var(--green-300);
            padding: 0.15rem 0.55rem;
            border-radius: 0.35rem;
            font-size: 0.68rem;
            border: 1px solid var(--border);
        }

        .news-card .news-h1 {
            font-size: 0.85rem;
            color: var(--green-200);
            font-weight: 600;
            margin-bottom: 0.5rem;
            padding-right: 0.75rem;
            border-right: 3px solid var(--green-600);
        }

        .news-card .news-body-preview {
            font-size: 0.78rem;
            color: var(--text-muted);
            line-height: 1.7;
            background: var(--bg-card);
            border-radius: 0.5rem;
            padding: 0.75rem 1rem;
            margin-top: 0.5rem;
            display: -webkit-box;
            -webkit-line-clamp: 4;
            -webkit-box-orient: vertical;
            overflow: hidden;
            border: 1px solid var(--border-light);
        }

        .news-card .news-image {
            width: 100%;
            max-height: 180px;
            object-fit: cover;
            border-radius: 0.5rem;
            margin-bottom: 0.75rem;
            border: 1px solid var(--border);
        }

        /* ───────────────────────── ACTIVITY LOG (SIDEBAR) ───────────────────────── */
        .log-container {
            padding: 0.75rem;
            max-height: 700px;
            overflow-y: auto;
        }

        .log-entry {
            padding: 0.65rem 0.85rem;
            border-radius: 0.5rem;
            margin-bottom: 0.4rem;
            font-size: 0.78rem;
            background: var(--bg-primary);
            border-right: 3px solid var(--border);
            transition: all 0.2s;
        }

        .log-entry:hover {
            background: rgba(10, 26, 15, 0.9);
        }

        .log-entry.page    { border-right-color: var(--success); }
        .log-entry.error   { border-right-color: var(--error); background: rgba(248, 113, 113, 0.05); }
        .log-entry.link    { border-right-color: var(--info); }
        .log-entry.status  { border-right-color: var(--warning); }
        .log-entry.sitemap { border-right-color: var(--sitemap); }

        .log-entry .log-time {
            color: var(--text-muted);
            font-size: 0.68rem;
            font-family: var(--font-mono);
            direction: ltr;
            display: inline-block;
        }

        .log-entry .log-type {
            display: inline-block;
            font-size: 0.65rem;
            font-weight: 700;
            padding: 0.1rem 0.4rem;
            border-radius: 0.25rem;
            margin-right: 0.4rem;
            text-transform: uppercase;
        }

        .log-entry.page .log-type    { background: rgba(74,222,128,0.15); color: var(--success); }
        .log-entry.error .log-type   { background: rgba(248,113,113,0.15); color: var(--error); }
        .log-entry.status .log-type  { background: rgba(251,191,36,0.15); color: var(--warning); }
        .log-entry.sitemap .log-type { background: rgba(167,139,250,0.15); color: var(--sitemap); }
        .log-entry.link .log-type    { background: rgba(56,189,248,0.15); color: var(--info); }

        .log-entry .log-msg {
            margin-top: 0.25rem;
            color: var(--text-secondary);
            word-break: break-all;
            font-size: 0.76rem;
        }

        .log-entry .log-msg a {
            color: var(--accent-dim);
            text-decoration: none;
        }
        .log-entry .log-msg a:hover {
            text-decoration: underline;
            color: var(--accent);
        }

        /* ───────────────────────── EMPTY STATES ───────────────────────── */
        .empty-state {
            text-align: center;
            padding: 3rem 1.5rem;
            color: var(--text-muted);
        }

        .empty-state .empty-icon {
            font-size: 3rem;
            margin-bottom: 1rem;
            opacity: 0.5;
        }

        .empty-state .empty-title {
            font-size: 1rem;
            color: var(--text-secondary);
            margin-bottom: 0.35rem;
        }

        .empty-state .empty-sub {
            font-size: 0.8rem;
        }

        /* ───────────────────────── TABS IN HEADER ───────────────────────── */
        .view-tabs {
            display: flex;
            gap: 0.25rem;
        }

        .view-tab {
            padding: 0.3rem 0.75rem;
            border-radius: 0.5rem;
            font-size: 0.75rem;
            color: var(--text-muted);
            cursor: pointer;
            border: 1px solid transparent;
            background: transparent;
            transition: all 0.2s;
        }

        .view-tab:hover {
            color: var(--text-secondary);
            background: var(--bg-primary);
        }

        .view-tab.active {
            color: var(--accent);
            background: rgba(74, 222, 128, 0.1);
            border-color: rgba(74, 222, 128, 0.2);
        }

        /* ───────────────────────── ANIMATIONS ───────────────────────── */
        @keyframes slideIn {
            from { opacity: 0; transform: translateY(-10px); }
            to   { opacity: 1; transform: translateY(0); }
        }

        @keyframes fadeIn {
            from { opacity: 0; }
            to   { opacity: 1; }
        }

        .news-card {
            animation: slideIn 0.3s ease-out;
        }

        .log-entry {
            animation: slideIn 0.2s ease-out;
        }

        /* ───────────────────────── FOOTER ───────────────────────── */
        .footer {
            text-align: center;
            padding: 1.5rem;
            color: var(--text-muted);
            font-size: 0.75rem;
            border-top: 1px solid var(--border-light);
            margin-top: 2rem;
        }

        .footer a {
            color: var(--accent-dim);
            text-decoration: none;
        }

        .footer a:hover {
            color: var(--accent);
        }
    </style>
</head>
<body>

<!-- ─────────────────── HEADER ─────────────────── -->
<header class="header">
    <div class="header-right">
        <div class="logo">
            <div class="logo-icon">🕷️</div>
            <div class="logo-text">
                <h1>Spiderly</h1>
                <span>خزنده هوشمند وب</span>
            </div>
        </div>
        <div id="target-url" class="target-url" style="display:none;"></div>
    </div>
    <div class="header-left">
        <div id="connection-status" class="status-badge connecting">
            <span class="status-dot"></span>
            <span>در حال اتصال...</span>
        </div>
    </div>
</header>

<!-- ─────────────────── MAIN ─────────────────── -->
<div class="container">

    <!-- Stats Row -->
    <div class="stats-grid">
        <div class="stat-card pages">
            <div class="stat-icon">📄</div>
            <div class="stat-value" id="stat-pages">0</div>
            <div class="stat-label">صفحات خزیده شده</div>
            <div class="stat-sub" id="stat-pages-sub">در انتظار شروع</div>
        </div>
        <div class="stat-card sitemap">
            <div class="stat-icon">🗺️</div>
            <div class="stat-value" id="stat-sitemap">0</div>
            <div class="stat-label">آدرس‌های نقشه سایت</div>
            <div class="stat-sub" id="stat-sitemap-sub">—</div>
        </div>
        <div class="stat-card links">
            <div class="stat-icon">🔗</div>
            <div class="stat-value" id="stat-links">0</div>
            <div class="stat-label">لینک‌های کشف شده</div>
            <div class="stat-sub" id="stat-links-sub">—</div>
        </div>
        <div class="stat-card errors">
            <div class="stat-icon">⚠️</div>
            <div class="stat-value" id="stat-errors">0</div>
            <div class="stat-label">خطاها</div>
            <div class="stat-sub" id="stat-errors-sub">—</div>
        </div>
        <div class="stat-card speed">
            <div class="stat-icon">⚡</div>
            <div class="stat-value" id="stat-speed">—</div>
            <div class="stat-label">سرعت (ص/ث)</div>
            <div class="stat-sub" id="stat-elapsed">00:00</div>
        </div>
    </div>

    <!-- Progress Bar -->
    <div class="progress-section" id="progress-section" style="display:none;">
        <div class="progress-info">
            <div class="label">پیشرفت خزش</div>
            <div class="percent" id="progress-pct">0%</div>
        </div>
        <div class="progress-bar-wrapper">
            <div class="progress-track">
                <div class="progress-fill" id="progress-fill"></div>
            </div>
        </div>
        <div class="progress-status" id="progress-text">0 / 0</div>
    </div>

    <!-- Main Grid: News + Activity Log -->
    <div class="main-grid">

        <!-- News / Scraped Pages Panel -->
        <div class="panel">
            <div class="panel-header">
                <h2>📰 اخبار و صفحات استخراج شده</h2>
                <div class="panel-controls">
                    <div class="view-tabs">
                        <button class="view-tab active" data-view="cards" onclick="setView('cards')">کارت</button>
                        <button class="view-tab" data-view="compact" onclick="setView('compact')">فشرده</button>
                    </div>
                    <button class="panel-btn" onclick="clearNews()">پاک کردن</button>
                </div>
            </div>
            <div class="news-container" id="news-container">
                <div class="empty-state">
                    <div class="empty-icon">📰</div>
                    <div class="empty-title">هنوز صفحه‌ای خزیده نشده</div>
                    <div class="empty-sub">بعد از شروع خزش، اخبار و جزئیات صفحات اینجا نمایش داده می‌شود</div>
                </div>
            </div>
        </div>

        <!-- Activity Log Panel -->
        <div class="panel">
            <div class="panel-header">
                <h2>📋 گزارش فعالیت</h2>
                <span class="panel-badge" id="log-count">0</span>
            </div>
            <div class="log-container" id="log-container">
                <div class="empty-state">
                    <div class="empty-icon">🕷️</div>
                    <div class="empty-title">در انتظار فعالیت...</div>
                    <div class="empty-sub">فعالیت‌های خزش به‌صورت زنده اینجا نمایش داده می‌شود</div>
                </div>
            </div>
        </div>

    </div>
</div>

<!-- Footer -->
<div class="footer">
    🕷️ Spiderly v1.0 — خزنده هوشمند با داشبورد بلادرنگ
</div>

<!-- ─────────────────── JAVASCRIPT ─────────────────── -->
<script>
(function() {
    "use strict";

    // ── DOM refs ──
    const newsContainer  = document.getElementById("news-container");
    const logContainer   = document.getElementById("log-container");
    const statusBadge    = document.getElementById("connection-status");
    const logCountBadge  = document.getElementById("log-count");
    const targetUrlEl    = document.getElementById("target-url");
    const progressSection = document.getElementById("progress-section");
    const progressFill   = document.getElementById("progress-fill");
    const progressPct    = document.getElementById("progress-pct");
    const progressText   = document.getElementById("progress-text");

    const statPages      = document.getElementById("stat-pages");
    const statSitemap    = document.getElementById("stat-sitemap");
    const statLinks      = document.getElementById("stat-links");
    const statErrors     = document.getElementById("stat-errors");
    const statSpeed      = document.getElementById("stat-speed");
    const statElapsed    = document.getElementById("stat-elapsed");
    const statPagesSub   = document.getElementById("stat-pages-sub");
    const statSitemapSub = document.getElementById("stat-sitemap-sub");
    const statErrorsSub  = document.getElementById("stat-errors-sub");

    // ── State ──
    let stats = { pages: 0, sitemap: 0, sitemapTotal: 0, links: 0, errors: 0, maxPages: 0 };
    let logCount = 0;
    let startTime = null;
    let currentView = "cards";
    let speedInterval = null;

    // ── Helpers ──
    function escapeHtml(str) {
        if (!str) return "";
        const d = document.createElement("div");
        d.appendChild(document.createTextNode(str));
        return d.innerHTML;
    }

    function truncate(str, max) {
        if (!str) return "";
        return str.length > max ? str.substring(0, max) + "..." : str;
    }

    function formatElapsed(ms) {
        const secs = Math.floor(ms / 1000);
        const m = Math.floor(secs / 60);
        const s = secs % 60;
        return (m < 10 ? "0" : "") + m + ":" + (s < 10 ? "0" : "") + s;
    }

    function getStatusClass(code) {
        if (code >= 200 && code < 300) return "s200";
        if (code >= 300 && code < 400) return "s301";
        return "s400";
    }

    // ── Connection Status ──
    function setStatus(state, text) {
        statusBadge.className = "status-badge " + state;
        statusBadge.innerHTML = '<span class="status-dot"></span><span>' + text + '</span>';
    }

    // ── View Toggle ──
    window.setView = function(view) {
        currentView = view;
        document.querySelectorAll(".view-tab").forEach(function(t) {
            t.classList.toggle("active", t.dataset.view === view);
        });
        // Re-render existing cards with new view
        var cards = newsContainer.querySelectorAll(".news-card");
        cards.forEach(function(c) {
            if (view === "compact") {
                c.classList.add("compact-view");
            } else {
                c.classList.remove("compact-view");
            }
        });
    };

    window.clearNews = function() {
        newsContainer.innerHTML = '<div class="empty-state"><div class="empty-icon">📰</div><div class="empty-title">پاک شد</div><div class="empty-sub">در انتظار صفحات جدید...</div></div>';
    };

    // ── Stats Update ──
    function updateStats() {
        statPages.textContent   = stats.pages.toLocaleString("fa-IR");
        statSitemap.textContent = stats.sitemap.toLocaleString("fa-IR");
        statLinks.textContent   = stats.links.toLocaleString("fa-IR");
        statErrors.textContent  = stats.errors.toLocaleString("fa-IR");

        if (stats.errors > 0) {
            statErrorsSub.textContent = "نرخ خطا: " + ((stats.errors / Math.max(stats.pages, 1)) * 100).toFixed(1) + "%";
        }

        if (stats.maxPages > 0) {
            var pct = Math.min((stats.pages / stats.maxPages) * 100, 100);
            progressSection.style.display = "flex";
            progressFill.style.width = pct + "%";
            progressPct.textContent = pct.toFixed(0) + "%";
            progressText.textContent = stats.pages + " / " + stats.maxPages;
        }
    }

    function startSpeedTracker() {
        if (speedInterval) return;
        startTime = Date.now();
        speedInterval = setInterval(function() {
            var elapsed = Date.now() - startTime;
            var pps = (stats.pages / (elapsed / 1000)).toFixed(1);
            statSpeed.textContent = pps;
            statElapsed.textContent = formatElapsed(elapsed);
        }, 1000);
    }

    function stopSpeedTracker() {
        if (speedInterval) {
            clearInterval(speedInterval);
            speedInterval = null;
        }
    }

    // ── Add Log Entry ──
    function addLog(type, message) {
        if (logContainer.querySelector(".empty-state")) {
            logContainer.innerHTML = "";
        }

        logCount++;
        logCountBadge.textContent = logCount;

        var typeLabels = {
            page: "صفحه", error: "خطا", status: "وضعیت", sitemap: "نقشه", link: "لینک"
        };

        var el = document.createElement("div");
        el.className = "log-entry " + type;

        var now = new Date().toLocaleTimeString("fa-IR");

        el.innerHTML =
            '<div style="display:flex;justify-content:space-between;align-items:center;">' +
              '<span class="log-type">' + (typeLabels[type] || type) + '</span>' +
              '<span class="log-time">' + now + '</span>' +
            '</div>' +
            '<div class="log-msg">' + message + '</div>';

        logContainer.prepend(el);

        // Keep max 200 log entries
        while (logContainer.children.length > 200) {
            logContainer.removeChild(logContainer.lastChild);
        }
    }

    // ── Add News Card ──
    function addNewsCard(data) {
        if (newsContainer.querySelector(".empty-state")) {
            newsContainer.innerHTML = "";
        }

        var card = document.createElement("div");
        card.className = "news-card" + (currentView === "compact" ? " compact-view" : "");

        var html = "";

        // Top row: status code + date
        html += '<div class="news-card-top">';
        html += '  <div>';
        if (data.title) {
            html += '    <div class="news-title">' + escapeHtml(data.title) + '</div>';
        } else {
            html += '    <div class="news-title" style="color:var(--text-muted);font-style:italic;">بدون عنوان</div>';
        }
        html += '  </div>';
        if (data.status_code) {
            html += '  <span class="news-status ' + getStatusClass(data.status_code) + '">⬤ ' + data.status_code + '</span>';
        }
        html += '</div>';

        // URL
        if (data.url) {
            html += '<a class="news-url" href="' + escapeHtml(data.url) + '" target="_blank" rel="noopener">' + escapeHtml(data.url) + '</a>';
        }

        // OG Image
        if (data.og_image) {
            html += '<img class="news-image" src="' + escapeHtml(data.og_image) + '" alt="" loading="lazy" onerror="this.style.display=\'none\'">';
        }

        // H1
        if (data.h1 && data.h1 !== data.title) {
            html += '<div class="news-h1">' + escapeHtml(truncate(data.h1, 150)) + '</div>';
        }

        // Description
        if (data.description) {
            html += '<div class="news-description">' + escapeHtml(data.description) + '</div>';
        }

        // Metadata row
        var hasMeta = data.author || data.published_date || data.content_type || data.load_time_ms;
        if (hasMeta) {
            html += '<div class="news-meta">';
            if (data.author) {
                html += '<div class="news-meta-item"><span class="meta-icon">✍️</span><span class="meta-value">' + escapeHtml(data.author) + '</span></div>';
            }
            if (data.published_date) {
                html += '<div class="news-meta-item"><span class="meta-icon">📅</span><span class="meta-value">' + escapeHtml(data.published_date) + '</span></div>';
            }
            if (data.content_type) {
                html += '<div class="news-meta-item"><span class="meta-icon">📃</span><span class="meta-value" style="direction:ltr">' + escapeHtml(data.content_type) + '</span></div>';
            }
            if (data.load_time_ms) {
                html += '<div class="news-meta-item"><span class="meta-icon">⏱️</span><span class="meta-value" style="direction:ltr">' + data.load_time_ms + 'ms</span></div>';
            }
            if (data.links_count) {
                html += '<div class="news-meta-item"><span class="meta-icon">🔗</span><span class="meta-value">' + data.links_count + ' لینک</span></div>';
            }
            if (data.images_count) {
                html += '<div class="news-meta-item"><span class="meta-icon">🖼️</span><span class="meta-value">' + data.images_count + ' تصویر</span></div>';
            }
            html += '</div>';
        }

        // Keywords as tags
        if (data.keywords) {
            var tags = data.keywords.split(",");
            if (tags.length > 0 && tags[0].trim()) {
                html += '<div class="news-tags">';
                for (var i = 0; i < Math.min(tags.length, 8); i++) {
                    var tag = tags[i].trim();
                    if (tag) {
                        html += '<span class="news-tag">' + escapeHtml(tag) + '</span>';
                    }
                }
                if (tags.length > 8) {
                    html += '<span class="news-tag" style="color:var(--text-muted);">+' + (tags.length - 8) + '</span>';
                }
                html += '</div>';
            }
        }

        // Body text preview
        if (data.body_text && data.body_text.trim().length > 20) {
            html += '<div class="news-body-preview">' + escapeHtml(truncate(data.body_text.trim(), 400)) + '</div>';
        }

        card.innerHTML = html;
        newsContainer.prepend(card);

        // Keep max 100 cards
        while (newsContainer.children.length > 100) {
            newsContainer.removeChild(newsContainer.lastChild);
        }
    }

    // ── WebSocket ──
    function connect() {
        var protocol = location.protocol === "https:" ? "wss://" : "ws://";
        var ws = new WebSocket(protocol + location.host + "/ws");

        ws.onopen = function() {
            setStatus("connected", "متصل");
            addLog("status", "اتصال WebSocket برقرار شد ✓");
        };

        ws.onclose = function() {
            setStatus("disconnected", "قطع شده");
            addLog("error", "اتصال قطع شد — تلاش مجدد...");
            stopSpeedTracker();
            setTimeout(connect, 2000);
        };

        ws.onerror = function() {
            addLog("error", "خطای WebSocket");
        };

        ws.onmessage = function(event) {
            var msg;
            try { msg = JSON.parse(event.data); } catch(e) { return; }
            var d = msg.data || {};

            switch (msg.type) {

                case "status":
                    addLog("status", escapeHtml(d.message || ""));
                    if (d.target_url) {
                        targetUrlEl.style.display = "block";
                        targetUrlEl.textContent = d.target_url;
                    }
                    break;

                case "strategy":
                    addLog("status", "حالت خزش: <strong>" + escapeHtml(d.mode || "") + "</strong>");
                    break;

                case "config":
                    if (d.max_pages) {
                        stats.maxPages = d.max_pages;
                    }
                    break;

                case "sitemap_stats":
                    stats.sitemap = d.filteredUrls || d.filtered_urls || 0;
                    stats.sitemapTotal = d.totalUrls || d.total_urls || 0;
                    updateStats();
                    statSitemapSub.textContent = "از " + stats.sitemapTotal.toLocaleString("fa-IR") + " مجموع";
                    addLog("sitemap", "کشف " + stats.sitemapTotal + " آدرس نقشه سایت — " + stats.sitemap + " بعد از فیلتر");
                    if (stats.maxPages === 0 && stats.sitemap > 0) {
                        stats.maxPages = stats.sitemap;
                    }
                    break;

                case "page":
                    stats.pages++;
                    startSpeedTracker();
                    updateStats();
                    statPagesSub.textContent = "آخرین: " + truncate(d.url || "", 35);
                    addLog("page", '<a href="' + escapeHtml(d.url) + '" target="_blank">' + escapeHtml(truncate(d.url || "", 60)) + '</a>' + (d.title ? " — " + escapeHtml(truncate(d.title, 50)) : ""));
                    addNewsCard(d);
                    break;

                case "link":
                    stats.links++;
                    updateStats();
                    break;

                case "error":
                    stats.errors++;
                    updateStats();
                    addLog("error", escapeHtml(d.url || "") + " — " + escapeHtml(d.error || "unknown"));
                    break;

                case "complete":
                    stopSpeedTracker();
                    addLog("status", "✅ خزش کامل شد — " + escapeHtml(d.duration || ""));
                    statPagesSub.textContent = "تکمیل شد ✅";
                    if (stats.maxPages > 0) {
                        progressFill.style.width = "100%";
                        progressPct.textContent = "100%";
                    }
                    break;
            }
        };
    }

    connect();

})();
</script>

</body>
</html>`
