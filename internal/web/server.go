package web

import (
	"fmt"
	"log"
	"net/http"
)

// Server holds the HTTP server and WebSocket hub
type Server struct {
	Hub  *Hub
	Port int
}

// NewServer creates a new web server
func NewServer(port int) *Server {
	hub := NewHub()

	return &Server{
		Hub:  hub,
		Port: port,
	}
}

// Start begins serving HTTP and WebSocket connections
func (s *Server) Start() {
	go s.Hub.Run()

	// Serve the HTML dashboard
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(DashboardHTML))
	})

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		HandleWebSocket(s.Hub, w, r)
	})

	addr := fmt.Sprintf(":%d", s.Port)
	log.Printf("🌐 Dashboard: http://localhost%s", addr)
	log.Printf("🔌 WebSocket: ws://localhost%s/ws", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
}

// DashboardHTML is the full HTML for the live dashboard
var DashboardHTML = `<!DOCTYPE html>
<html lang="fa" dir="rtl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Spiderly - داشبورد خزشگر وب</title>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Vazirmatn:wght@300;400;500;700;900&display=swap');
        @import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap');

        :root {
            --bg-primary: #0a0e17;
            --bg-secondary: #111827;
            --bg-card: #1a2332;
            --bg-card-hover: #1f2b3d;
            --border: #1e3a2f;
            --border-glow: #00ff8855;
            --green-primary: #00ff88;
            --green-secondary: #00cc6a;
            --green-dim: #00ff8833;
            --green-dark: #003d20;
            --text-primary: #e0ffe8;
            --text-secondary: #88c0a0;
            --text-muted: #4a6858;
            --red: #ff4444;
            --red-dim: #ff444433;
            --yellow: #ffcc00;
            --yellow-dim: #ffcc0033;
            --blue: #00aaff;
            --blue-dim: #00aaff33;
            --font-persian: 'Vazirmatn', sans-serif;
            --font-mono: 'JetBrains Mono', monospace;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: var(--font-persian);
            background: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
            overflow-x: hidden;
        }

        /* Scrollbar */
        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-track { background: var(--bg-secondary); }
        ::-webkit-scrollbar-thumb { background: var(--green-dark); border-radius: 3px; }
        ::-webkit-scrollbar-thumb:hover { background: var(--green-secondary); }

        /* Scanline effect */
        body::before {
            content: '';
            position: fixed;
            top: 0; left: 0; right: 0; bottom: 0;
            background: repeating-linear-gradient(
                0deg,
                transparent,
                transparent 2px,
                rgba(0, 255, 136, 0.015) 2px,
                rgba(0, 255, 136, 0.015) 4px
            );
            pointer-events: none;
            z-index: 9999;
        }

        /* Header */
        .header {
            background: linear-gradient(180deg, var(--bg-secondary) 0%, var(--bg-primary) 100%);
            border-bottom: 1px solid var(--border);
            padding: 20px 30px;
            position: sticky;
            top: 0;
            z-index: 100;
            backdrop-filter: blur(10px);
        }

        .header-content {
            max-width: 1400px;
            margin: 0 auto;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }

        .logo {
            display: flex;
            align-items: center;
            gap: 15px;
        }

        .logo-icon {
            font-size: 36px;
            filter: drop-shadow(0 0 10px var(--green-primary));
        }

        .logo-text {
            font-family: var(--font-mono);
            font-size: 28px;
            font-weight: 900;
            color: var(--green-primary);
            text-shadow: 0 0 20px var(--green-dim);
            letter-spacing: 3px;
        }

        .logo-sub {
            font-size: 12px;
            color: var(--text-muted);
            font-family: var(--font-persian);
            letter-spacing: 0;
        }

        .connection-status {
            display: flex;
            align-items: center;
            gap: 8px;
            font-family: var(--font-mono);
            font-size: 13px;
            padding: 8px 16px;
            border-radius: 20px;
            border: 1px solid var(--border);
        }

        .connection-status.connected {
            color: var(--green-primary);
            border-color: var(--green-dark);
            background: var(--green-dim);
        }

        .connection-status.disconnected {
            color: var(--red);
            border-color: #3d0000;
            background: var(--red-dim);
        }

        .status-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            animation: pulse 2s infinite;
        }

        .connected .status-dot { background: var(--green-primary); }
        .disconnected .status-dot { background: var(--red); animation: none; }

        @keyframes pulse {
            0%, 100% { opacity: 1; box-shadow: 0 0 0 0 var(--green-dim); }
            50% { opacity: 0.7; box-shadow: 0 0 0 8px transparent; }
        }

        /* Main Layout */
        .main {
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px 30px;
            display: grid;
            grid-template-columns: 1fr 350px;
            gap: 20px;
        }

        /* Stats Bar */
        .stats-bar {
            grid-column: 1 / -1;
            display: grid;
            grid-template-columns: repeat(5, 1fr);
            gap: 15px;
        }

        .stat-card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 20px;
            text-align: center;
            transition: all 0.3s ease;
            position: relative;
            overflow: hidden;
        }

        .stat-card::before {
            content: '';
            position: absolute;
            top: 0; left: 0; right: 0;
            height: 2px;
            background: var(--green-primary);
            opacity: 0.5;
        }

        .stat-card:hover {
            border-color: var(--green-dark);
            background: var(--bg-card-hover);
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(0, 255, 136, 0.1);
        }

        .stat-icon {
            font-size: 24px;
            margin-bottom: 8px;
        }

        .stat-value {
            font-family: var(--font-mono);
            font-size: 32px;
            font-weight: 700;
            color: var(--green-primary);
            text-shadow: 0 0 15px var(--green-dim);
        }

        .stat-label {
            font-size: 13px;
            color: var(--text-muted);
            margin-top: 5px;
        }

        /* News Feed */
        .news-feed {
            display: flex;
            flex-direction: column;
            gap: 15px;
        }

        .section-title {
            font-family: var(--font-mono);
            font-size: 16px;
            color: var(--green-primary);
            padding: 10px 0;
            border-bottom: 1px solid var(--border);
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .section-title span {
            font-family: var(--font-persian);
            color: var(--text-secondary);
        }

        .news-card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 20px;
            transition: all 0.3s ease;
            animation: slideIn 0.5s ease;
            position: relative;
        }

        .news-card::before {
            content: '';
            position: absolute;
            right: 0; top: 15px; bottom: 15px;
            width: 3px;
            background: var(--green-primary);
            border-radius: 3px;
            opacity: 0.6;
        }

        .news-card:hover {
            border-color: var(--green-dark);
            background: var(--bg-card-hover);
            box-shadow: 0 0 30px rgba(0, 255, 136, 0.05);
        }

        @keyframes slideIn {
            from { opacity: 0; transform: translateX(20px); }
            to { opacity: 1; transform: translateX(0); }
        }

        .news-index {
            position: absolute;
            top: -8px;
            left: 15px;
            background: var(--green-primary);
            color: var(--bg-primary);
            font-family: var(--font-mono);
            font-weight: 700;
            font-size: 12px;
            padding: 2px 10px;
            border-radius: 10px;
        }

        .news-title {
            font-size: 18px;
            font-weight: 700;
            color: var(--text-primary);
            margin-bottom: 12px;
            line-height: 1.8;
        }

        .news-content {
            font-size: 14px;
            color: var(--text-secondary);
            line-height: 2;
            margin-bottom: 15px;
            max-height: 120px;
            overflow: hidden;
            position: relative;
        }

        .news-content::after {
            content: '';
            position: absolute;
            bottom: 0; left: 0; right: 0;
            height: 40px;
            background: linear-gradient(transparent, var(--bg-card));
        }

        .news-meta {
            display: flex;
            flex-wrap: wrap;
            gap: 12px;
            font-size: 12px;
            font-family: var(--font-mono);
        }

        .meta-item {
            display: flex;
            align-items: center;
            gap: 5px;
            color: var(--text-muted);
            background: rgba(0,255,136,0.05);
            padding: 4px 10px;
            border-radius: 6px;
            border: 1px solid rgba(0,255,136,0.1);
        }

        .news-tags {
            display: flex;
            flex-wrap: wrap;
            gap: 6px;
            margin-top: 12px;
        }

        .tag {
            background: var(--green-dim);
            color: var(--green-primary);
            font-size: 11px;
            padding: 3px 10px;
            border-radius: 12px;
            border: 1px solid var(--green-dark);
            font-family: var(--font-persian);
        }

        .news-url {
            display: block;
            color: var(--blue);
            font-size: 12px;
            font-family: var(--font-mono);
            margin-top: 10px;
            text-decoration: none;
            word-break: break-all;
            opacity: 0.7;
            transition: opacity 0.2s;
        }

        .news-url:hover { opacity: 1; }

        /* Sidebar */
        .sidebar {
            display: flex;
            flex-direction: column;
            gap: 15px;
        }

        /* Log Panel */
        .log-panel {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 12px;
            overflow: hidden;
            max-height: 400px;
        }

        .log-header {
            background: var(--bg-secondary);
            padding: 12px 16px;
            display: flex;
            align-items: center;
            gap: 8px;
            border-bottom: 1px solid var(--border);
        }

        .log-header-dot {
            width: 10px; height: 10px;
            border-radius: 50%;
        }

        .log-body {
            padding: 12px;
            font-family: var(--font-mono);
            font-size: 12px;
            overflow-y: auto;
            max-height: 340px;
            direction: ltr;
        }

        .log-entry {
            padding: 4px 0;
            border-bottom: 1px solid rgba(255,255,255,0.03);
            display: flex;
            gap: 8px;
            animation: fadeIn 0.3s ease;
        }

        @keyframes fadeIn {
            from { opacity: 0; }
            to { opacity: 1; }
        }

        .log-time {
            color: var(--text-muted);
            white-space: nowrap;
            min-width: 70px;
        }

        .log-level {
            font-weight: 700;
            min-width: 40px;
            text-align: center;
            border-radius: 3px;
            padding: 0 4px;
        }

        .log-level.info { color: var(--green-primary); }
        .log-level.warn { color: var(--yellow); }
        .log-level.error { color: var(--red); }
        .log-level.success { color: var(--green-primary); }

        .log-msg {
            color: var(--text-secondary);
            word-break: break-all;
        }

        /* Links Panel */
        .links-panel {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 12px;
            overflow: hidden;
            max-height: 350px;
        }

        .links-body {
            padding: 12px;
            overflow-y: auto;
            max-height: 300px;
        }

        .link-item {
            padding: 8px 10px;
            border-bottom: 1px solid rgba(255,255,255,0.03);
            font-size: 12px;
            animation: fadeIn 0.3s ease;
        }

        .link-text {
            color: var(--text-secondary);
            font-family: var(--font-persian);
            margin-bottom: 3px;
        }

        .link-url {
            color: var(--blue);
            font-family: var(--font-mono);
            font-size: 11px;
            opacity: 0.6;
            word-break: break-all;
            text-decoration: none;
        }

        .link-url:hover { opacity: 1; }

        .link-depth {
            display: inline-block;
            background: var(--blue-dim);
            color: var(--blue);
            font-family: var(--font-mono);
            font-size: 10px;
            padding: 1px 6px;
            border-radius: 4px;
            margin-left: 5px;
        }

        /* Progress Bar */
        .progress-container {
            grid-column: 1 / -1;
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 15px 20px;
        }

        .progress-info {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 10px;
        }

        .progress-url {
            font-family: var(--font-mono);
            font-size: 12px;
            color: var(--text-muted);
            max-width: 70%;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            direction: ltr;
        }

        .progress-percent {
            font-family: var(--font-mono);
            font-size: 14px;
            font-weight: 700;
            color: var(--green-primary);
        }

        .progress-bar-bg {
            height: 4px;
            background: var(--bg-secondary);
            border-radius: 2px;
            overflow: hidden;
        }

        .progress-bar-fill {
            height: 100%;
            background: linear-gradient(90deg, var(--green-secondary), var(--green-primary));
            border-radius: 2px;
            transition: width 0.5s ease;
            box-shadow: 0 0 10px var(--green-dim);
        }

        /* Status overlay */
        .status-overlay {
            grid-column: 1 / -1;
            text-align: center;
            padding: 60px;
            color: var(--text-muted);
        }

        .status-overlay .waiting-icon {
            font-size: 64px;
            margin-bottom: 20px;
            animation: float 3s ease-in-out infinite;
        }

        @keyframes float {
            0%, 100% { transform: translateY(0); }
            50% { transform: translateY(-10px); }
        }

        .status-overlay h2 {
            color: var(--green-primary);
            font-size: 22px;
            margin-bottom: 10px;
        }

        .empty-state {
            text-align: center;
            padding: 30px;
            color: var(--text-muted);
            font-size: 13px;
        }

        /* Responsive */
        @media (max-width: 900px) {
            .main { grid-template-columns: 1fr; }
            .stats-bar { grid-template-columns: repeat(3, 1fr); }
            .header-content { flex-direction: column; gap: 10px; }
        }

        @media (max-width: 600px) {
            .stats-bar { grid-template-columns: repeat(2, 1fr); }
            .main { padding: 10px 15px; }
        }
    </style>
</head>
<body>
    <header class="header">
        <div class="header-content">
            <div class="logo">
                <div class="logo-icon">🕷️</div>
                <div>
                    <div class="logo-text">SPIDERLY</div>
                    <div class="logo-sub">خزشگر هوشمند وب</div>
                </div>
            </div>
            <div id="connectionStatus" class="connection-status disconnected">
                <div class="status-dot"></div>
                <span id="connectionText">در انتظار اتصال...</span>
            </div>
        </div>
    </header>

    <main class="main">
        <!-- Stats Bar -->
        <div class="stats-bar">
            <div class="stat-card">
                <div class="stat-icon">📄</div>
                <div class="stat-value" id="statPages">0</div>
                <div class="stat-label">صفحات بررسی شده</div>
            </div>
            <div class="stat-card">
                <div class="stat-icon">📰</div>
                <div class="stat-value" id="statNews">0</div>
                <div class="stat-label">اخبار استخراج شده</div>
            </div>
            <div class="stat-card">
                <div class="stat-icon">🔗</div>
                <div class="stat-value" id="statLinks">0</div>
                <div class="stat-label">لینک‌ کشف شده</div>
            </div>
            <div class="stat-card">
                <div class="stat-icon">❌</div>
                <div class="stat-value" id="statErrors" style="color: var(--red);">0</div>
                <div class="stat-label">خطاها</div>
            </div>
            <div class="stat-card">
                <div class="stat-icon">⏱️</div>
                <div class="stat-value" id="statTime" style="font-size: 20px;">00:00</div>
                <div class="stat-label">زمان سپری شده</div>
            </div>
        </div>

        <!-- Progress Bar -->
        <div class="progress-container" id="progressContainer" style="display:none;">
            <div class="progress-info">
                <div class="progress-url" id="progressURL">در انتظار...</div>
                <div class="progress-percent" id="progressPercent">0%</div>
            </div>
            <div class="progress-bar-bg">
                <div class="progress-bar-fill" id="progressBar" style="width: 0%"></div>
            </div>
        </div>

        <!-- News Feed -->
        <div class="news-feed">
            <div class="section-title">
                > <span>اخبار استخراج شده</span>
            </div>
            <div id="newsFeed">
                <div class="status-overlay" id="waitingState">
                    <div class="waiting-icon">🕷️</div>
                    <h2>در انتظار شروع خزش...</h2>
                    <p>خزشگر به زودی شروع به کار می‌کند</p>
                </div>
            </div>
        </div>

        <!-- Sidebar -->
        <div class="sidebar">
            <!-- Log Panel -->
            <div class="log-panel">
                <div class="log-header">
                    <div class="log-header-dot" style="background:#ff5f57"></div>
                    <div class="log-header-dot" style="background:#ffbd2e"></div>
                    <div class="log-header-dot" style="background:#28c840"></div>
                    <span style="margin-right:10px; font-family: var(--font-mono); font-size:13px; color: var(--text-muted);">terminal</span>
                </div>
                <div class="log-body" id="logBody">
                    <div class="empty-state">در انتظار لاگ‌ها...</div>
                </div>
            </div>

            <!-- Links Panel -->
            <div class="links-panel">
                <div class="log-header">
                    <span style="font-family: var(--font-mono); font-size:13px; color: var(--text-muted);">🔗 لینک‌های کشف شده</span>
                </div>
                <div class="links-body" id="linksBody">
                    <div class="empty-state">هنوز لینکی کشف نشده...</div>
                </div>
            </div>
        </div>
    </main>

    <script>
        let ws;
        let newsCount = 0;
        let firstLog = true;
        let firstLink = true;
        let timerInterval;
        let startTime;

        function connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

            ws.onopen = () => {
                document.getElementById('connectionStatus').className = 'connection-status connected';
                document.getElementById('connectionText').textContent = 'متصل';
            };

            ws.onclose = () => {
                document.getElementById('connectionStatus').className = 'connection-status disconnected';
                document.getElementById('connectionText').textContent = 'قطع شده';
                setTimeout(connect, 3000);
            };

            ws.onerror = () => {
                ws.close();
            };

            ws.onmessage = (event) => {
                try {
                    const msg = JSON.parse(event.data);
                    handleMessage(msg);
                } catch (e) {
                    console.error('Parse error:', e);
                }
            };
        }

        function handleMessage(msg) {
            switch (msg.type) {
                case 'news':
                    addNews(msg.payload);
                    break;
                case 'log':
                    addLog(msg.payload);
                    break;
                case 'stats':
                    updateStats(msg.payload);
                    break;
                case 'progress':
                    updateProgress(msg.payload);
                    break;
                case 'link':
                    addLink(msg.payload);
                    break;
                case 'started':
                    onCrawlStarted();
                    break;
                case 'finished':
                    onCrawlFinished(msg.payload);
                    break;
            }
        }

        function addNews(news) {
            const waiting = document.getElementById('waitingState');
            if (waiting) waiting.remove();

            newsCount++;
            const feed = document.getElementById('newsFeed');

            let tagsHTML = '';
            if (news.tags && news.tags.length > 0) {
                tagsHTML = '<div class="news-tags">' +
                    news.tags.map(t => '<span class="tag">' + escapeHtml(t) + '</span>').join('') +
                    '</div>';
            }

            const card = document.createElement('div');
            card.className = 'news-card';
            card.innerHTML =
                '<div class="news-index">' + newsCount + '</div>' +
                '<div class="news-title">' + escapeHtml(news.title || 'بدون عنوان') + '</div>' +
                (news.content ? '<div class="news-content">' + escapeHtml(news.content) + '</div>' : '') +
                '<div class="news-meta">' +
                    (news.author ? '<div class="meta-item">✍️ ' + escapeHtml(news.author) + '</div>' : '') +
                    (news.published_at ? '<div class="meta-item">📅 ' + escapeHtml(news.published_at) + '</div>' : '') +
                '</div>' +
                tagsHTML +
                '<a href="' + escapeHtml(news.url) + '" target="_blank" class="news-url">' + escapeHtml(news.url) + '</a>';

            feed.insertBefore(card, feed.firstChild);
            document.getElementById('statNews').textContent = newsCount;
        }

        function addLog(log) {
            const body = document.getElementById('logBody');
            if (firstLog) {
                body.innerHTML = '';
                firstLog = false;
            }

            const entry = document.createElement('div');
            entry.className = 'log-entry';
            entry.innerHTML =
                '<span class="log-time">' + escapeHtml(log.timestamp || '') + '</span>' +
                '<span class="log-level ' + (log.level || 'info') + '">' + escapeHtml((log.level || 'info').toUpperCase()) + '</span>' +
                '<span class="log-msg">' + escapeHtml(log.message) + '</span>';

            body.appendChild(entry);
            body.scrollTop = body.scrollHeight;
        }

        function addLink(link) {
            const body = document.getElementById('linksBody');
            if (firstLink) {
                body.innerHTML = '';
                firstLink = false;
            }

            const item = document.createElement('div');
            item.className = 'link-item';
            item.innerHTML =
                '<div class="link-text">' + escapeHtml(link.text || 'بدون عنوان') +
                '<span class="link-depth">عمق: ' + (link.depth || 0) + '</span></div>' +
                '<a href="' + escapeHtml(link.url) + '" target="_blank" class="link-url">' + escapeHtml(link.url) + '</a>';

            body.appendChild(item);

            const count = body.querySelectorAll('.link-item').length;
            document.getElementById('statLinks').textContent = count;

            // Keep max 100 links visible
            while (body.children.length > 100) {
                body.removeChild(body.firstChild);
            }

            body.scrollTop = body.scrollHeight;
        }

        function updateStats(stats) {
            if (stats.total_pages !== undefined) document.getElementById('statPages').textContent = stats.total_pages;
            if (stats.total_news !== undefined) document.getElementById('statNews').textContent = stats.total_news;
            if (stats.total_links !== undefined) document.getElementById('statLinks').textContent = stats.total_links;
            if (stats.errors !== undefined) document.getElementById('statErrors').textContent = stats.errors;
            if (stats.elapsed_time) document.getElementById('statTime').textContent = stats.elapsed_time;
        }

        function updateProgress(progress) {
            const container = document.getElementById('progressContainer');
            container.style.display = 'block';

            document.getElementById('progressURL').textContent = progress.current_url || '';
            const pct = Math.round(progress.progress || 0);
            document.getElementById('progressPercent').textContent = pct + '%';
            document.getElementById('progressBar').style.width = pct + '%';
        }

        function onCrawlStarted() {
            startTime = Date.now();
            timerInterval = setInterval(() => {
                const elapsed = Math.floor((Date.now() - startTime) / 1000);
                const min = String(Math.floor(elapsed / 60)).padStart(2, '0');
                const sec = String(elapsed % 60).padStart(2, '0');
                document.getElementById('statTime').textContent = min + ':' + sec;
            }, 1000);
        }

        function onCrawlFinished(stats) {
            if (timerInterval) clearInterval(timerInterval);
            if (stats) updateStats(stats);

            document.getElementById('progressBar').style.width = '100%';
            document.getElementById('progressPercent').textContent = '100%';
            document.getElementById('progressURL').textContent = '✅ خزش به پایان رسید!';
        }

        function escapeHtml(str) {
            if (!str) return '';
            const div = document.createElement('div');
            div.textContent = str;
            return div.innerHTML;
        }

        // Connect on load
        connect();
    </script>
</body>
</html>`
