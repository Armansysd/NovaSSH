// NovaSSH Enterprise PRO v4.0 Frontend Controller (Multi-Session SSH & SFTP Engine)

let servers = [];
let snippets = [];
let currentGroup = 'All';

// Multi-Session SSH Terminal State
const terminalSessions = new Map(); // serverId -> { id, name, host, term, fitAddon, ws, containerEl }
let activeSSHSessionId = null;
let isTerminalFullscreen = false;

// Multi-Session SFTP State
const sftpSessions = new Map(); // serverId -> { currentPath, parentPath }
let currentSFTPServerId = null;

document.addEventListener('DOMContentLoaded', () => {
  const savedTheme = localStorage.getItem('novassh_theme') || 'oled-black';
  applyTheme(savedTheme);
  const savedLang = localStorage.getItem('novassh_lang') || 'en';
  setLanguage(savedLang);

  loadServers();
  loadSnippets();

  const searchInput = document.getElementById('search-input');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      renderServers(e.target.value);
    });
  }

  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      searchInput?.focus();
    }
  });

  window.addEventListener('resize', () => {
    if (activeSSHSessionId) {
      const sess = terminalSessions.get(activeSSHSessionId);
      if (sess && sess.fitAddon) {
        sess.fitAddon.fit();
        if (sess.ws && sess.ws.readyState === WebSocket.OPEN) {
          sess.ws.send(JSON.stringify({
            type: 'resize',
            cols: sess.term.cols,
            rows: sess.term.rows
          }));
        }
      }
    }
  });

  setInterval(() => {
    loadServers(true);
  }, 30000);

  // Heartbeat to keep backend server alive while app window is open
  const sendHeartbeat = () => {
    fetch('/api/heartbeat').catch(() => {});
  };
  sendHeartbeat();
  setInterval(sendHeartbeat, 3000);

  // Notify backend to shut down immediately when native app window is closed
  window.addEventListener('beforeunload', () => {
    navigator.sendBeacon('/api/shutdown');
  });
});

function applyTheme(theme) {
  document.body.setAttribute('data-theme', theme);
  localStorage.setItem('novassh_theme', theme);
  const sel = document.getElementById('theme-select');
  if (sel) sel.value = theme;
}

// ---- Tab Switching (Clean Tailwind visibility, zero specificity bug) ----
function switchTab(tabName) {
  const tabs = ['servers', 'terminal', 'sftp', 'services', 'docker', 'ports', 'keys', 'cluster', 'snippets', 'monitor', 'settings'];
  tabs.forEach(t => {
    const sec = document.getElementById(`tab-${t}`);
    const nav = document.getElementById(`nav-${t}`);
    if (sec) {
      sec.classList.add('hidden');
      sec.style.display = 'none';
    }
    if (nav) nav.classList.remove('active');
  });

  const activeSec = document.getElementById(`tab-${tabName}`);
  const activeNav = document.getElementById(`nav-${tabName}`);
  if (activeSec) {
    activeSec.classList.remove('hidden');
    activeSec.style.display = 'flex';
  }
  if (activeNav) activeNav.classList.add('active');

  if (tabName === 'terminal') {
    setTimeout(() => {
      if (activeSSHSessionId) {
        const sess = terminalSessions.get(activeSSHSessionId);
        if (sess && sess.fitAddon) {
          sess.fitAddon.fit();
          sess.term.focus();
        }
      }
    }, 50);
  }

  if (tabName === 'cluster') {
    renderClusterCheckboxes();
  }
}

// ---- Load & Render Servers ----
async function loadServers(silent = false) {
  try {
    const res = await fetch('/api/servers');
    if (!res.ok) return;
    servers = await res.json();
    renderServers();
    updateServerDropdowns();
    updateStats();
  } catch (err) {
    if (!silent) console.error('Error loading servers:', err);
  }
}

function updateStats() {
  const totalEl = document.getElementById('stat-total-servers');
  const onlineEl = document.getElementById('stat-online-servers');
  const badgeEl = document.getElementById('server-count-badge');
  if (totalEl) totalEl.innerText = servers.length;
  if (badgeEl) badgeEl.innerText = servers.length;
  if (onlineEl) {
    const onlineCount = servers.filter(s => s.status === 'online').length;
    onlineEl.innerText = onlineCount;
  }
}

function filterGroup(group) {
  currentGroup = group;
  renderServers();
}

function renderServers(searchQuery = '') {
  const grid = document.getElementById('servers-grid');
  if (!grid) return;

  let list = [...servers];
  if (currentGroup !== 'All') {
    list = list.filter(s => s.group === currentGroup);
  }

  if (searchQuery.trim() !== '') {
    const q = searchQuery.toLowerCase();
    list = list.filter(s => 
      s.name.toLowerCase().includes(q) ||
      s.host.toLowerCase().includes(q) ||
      s.group.toLowerCase().includes(q) ||
      (s.tags && s.tags.some(t => t.toLowerCase().includes(q)))
    );
  }

  if (list.length === 0) {
    grid.innerHTML = `
      <div class="col-span-full ent-panel p-8 text-center space-y-3">
        <p class="text-sm text-[#8b95a5]" data-i18n="no_servers">No servers found in this group.</p>
        <button onclick="openServerModal()" class="btn-primary text-xs" data-i18n="add_first_srv">+ Add First Server</button>
      </div>
    `;
    return;
  }

  grid.innerHTML = list.map(s => {
    const isOnline = s.status === 'online';
    const statusDotClass = isOnline ? 'dot-online' : (s.status === 'offline' ? 'dot-offline' : 'dot-unknown');
    const statusText = isOnline ? `Online (${s.latency_ms || 1} ms)` : (s.status === 'offline' ? 'Offline' : 'Unknown');
    const statusTextColor = isOnline ? 'text-[#10b981]' : (s.status === 'offline' ? 'text-[#ef4444]' : 'text-[#8b95a5]');
    const tagsHTML = (s.tags || []).map(t => `<span class="tag-badge">#${t}</span>`).join(' ');

    return `
      <div class="ent-card p-4 flex flex-col justify-between space-y-3">
        <div>
          <div class="flex items-start justify-between">
            <div>
              <h3 class="text-sm font-bold text-white flex items-center gap-2">
                ${s.name}
              </h3>
              <p class="text-xs text-[#8b95a5] font-mono mt-0.5" dir="ltr">${s.username}@${s.host}:${s.port}</p>
            </div>
            <div class="flex items-center gap-1.5 text-xs font-mono ${statusTextColor} bg-[#05070a] px-2 py-1 rounded border border-[#212836]">
              <span class="status-dot ${statusDotClass}"></span>
              <span>${statusText}</span>
            </div>
          </div>

          <div class="mt-2 flex flex-wrap gap-1">
            ${tagsHTML}
          </div>

          ${s.notes ? `<p class="text-xs text-[#8b95a5] mt-2 line-clamp-1">${s.notes}</p>` : ''}
        </div>

        <div class="pt-2 border-t border-[#212836] flex items-center justify-between gap-1.5">
          <div class="flex items-center gap-1.5">
            <button onclick="openTerminalForServer('${s.id}')" class="btn-primary text-xs !px-2.5 !py-1" data-i18n="connect_term">
              SSH Terminal
            </button>
            <button onclick="openSFTPForServer('${s.id}')" class="btn-default text-xs !px-2.5 !py-1" data-i18n="connect_sftp">
              SFTP Files
            </button>
            <button onclick="openMonitorForServer('${s.id}')" class="btn-default text-xs !px-2.5 !py-1" data-i18n="connect_monitor">
              Telemetry
            </button>
          </div>

          <div class="flex items-center gap-1">
            <button onclick="editServer('${s.id}')" class="btn-default !p-1.5" title="Edit">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"></path></svg>
            </button>
            <button onclick="deleteServer('${s.id}')" class="btn-default !p-1.5 text-[#ef4444] hover:border-[#ef4444]" title="Delete">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
            </button>
          </div>
        </div>
      </div>
    `;
  }).join('');
}

function updateServerDropdowns() {
  const ids = [
    'terminal-server-select', 'sftp-server-select', 'monitor-server-select',
    'services-server-select', 'docker-server-select', 'ports-server-select',
    'authkeys-server-select'
  ];

  const optsHTML = `<option value="">+ Connect Server...</option>` + servers.map(s => `
    <option value="${s.id}">${s.name} (${s.host}:${s.port})</option>
  `).join('');

  ids.forEach(id => {
    const el = document.getElementById(id);
    if (el) {
      const cur = el.value;
      el.innerHTML = optsHTML;
      if (cur) el.value = cur;
    }
  });
}

// ============================================================================
//  MULTI-CONNECTION / MULTI-SESSION SSH TERMINAL ENGINE
// ============================================================================

function openTerminalForServer(id) {
  switchTab('terminal');
  openNewSSHConnection(id);
}

function openNewSSHConnection(serverId) {
  if (!serverId) return;
  const srv = servers.find(s => s.id === serverId);
  if (!srv) {
    alert('Server not found.');
    return;
  }

  // If already open, simply switch to its tab without reconnecting
  if (terminalSessions.has(serverId)) {
    switchSSHTab(serverId);
    return;
  }

  // Create a new session DOM container inside #ssh-sessions-container
  const containerWrapper = document.getElementById('ssh-sessions-container');
  if (!containerWrapper) return;

  const sessionEl = document.createElement('div');
  sessionEl.id = `term-view-${serverId}`;
  sessionEl.className = 'terminal-viewport-box';
  sessionEl.style.display = 'none'; // Will show when switched to
  containerWrapper.appendChild(sessionEl);

  const themes = getTerminalColorThemes();
  const selectedColor = document.getElementById('term-color-select')?.value || 'warp';
  const selectedSize = parseInt(document.getElementById('term-fontsize-select')?.value) || 13;

  const term = new Terminal({
    cursorBlink: true,
    fontSize: selectedSize,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Consolas', 'Courier New', monospace",
    theme: themes[selectedColor] || themes.warp,
    scrollback: 10000,
    allowTransparency: false,
    tabStopWidth: 4
  });

  let fitAddon = null;
  if (window.FitAddon) {
    fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
  }

  term.open(sessionEl);

  // Initialize WebSocket connection for this specific session
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${proto}//${window.location.host}/api/ws/terminal?serverId=${encodeURIComponent(serverId)}`;
  const ws = new WebSocket(url);

  const sessionObj = {
    id: serverId,
    name: srv.name,
    host: srv.host,
    term: term,
    fitAddon: fitAddon,
    ws: ws,
    containerEl: sessionEl
  };

  terminalSessions.set(serverId, sessionObj);

  ws.onopen = () => {
    if (fitAddon) {
      setTimeout(() => {
        fitAddon.fit();
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({
            type: 'resize',
            cols: term.cols,
            rows: term.rows
          }));
        }
      }, 50);
    }
    updateSSHTabsBar();
  };

  ws.onmessage = (event) => {
    term.write(event.data);
  };

  ws.onclose = () => {
    term.write('\r\n\x1b[31;1m=== SSH Session Terminated ===\x1b[0m\r\n');
    updateSSHTabsBar();
  };

  ws.onerror = () => {
    term.write('\r\n\x1b[31;1m[Error] WebSocket connection failed\x1b[0m\r\n');
  };

  term.onData(data => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: 'input',
        data: data
      }));
    }
  });

  sessionEl.addEventListener('click', () => {
    term.focus();
    sessionEl.classList.add('terminal-focused-ring');
    setTimeout(() => {
      sessionEl.classList.remove('terminal-focused-ring');
    }, 1500);
  });

  // Switch to the newly created session tab immediately
  switchSSHTab(serverId);
}

function switchSSHTab(serverId) {
  if (!terminalSessions.has(serverId)) return;
  activeSSHSessionId = serverId;

  // Hide all session viewports, show the selected one
  terminalSessions.forEach((sess, id) => {
    if (id === serverId) {
      sess.containerEl.style.display = 'block';
      if (sess.fitAddon) sess.fitAddon.fit();
      sess.term.focus();
    } else {
      sess.containerEl.style.display = 'none';
    }
  });

  const activeSess = terminalSessions.get(serverId);
  const titleEl = document.getElementById('term-active-title');
  if (titleEl && activeSess) {
    titleEl.innerText = `${activeSess.name} (${activeSess.host})`;
  }

  const dot = document.getElementById('term-status-dot');
  if (dot && activeSess) {
    const isOpen = activeSess.ws && activeSess.ws.readyState === WebSocket.OPEN;
    dot.className = isOpen ? 'status-dot dot-online' : 'status-dot dot-offline';
  }

  const sel = document.getElementById('terminal-server-select');
  if (sel) sel.value = serverId;

  updateSSHTabsBar();
}

function closeSSHSession(serverId, event) {
  if (event) event.stopPropagation();
  const sess = terminalSessions.get(serverId);
  if (!sess) return;

  if (sess.ws) {
    sess.ws.close();
  }
  if (sess.containerEl && sess.containerEl.parentNode) {
    sess.containerEl.parentNode.removeChild(sess.containerEl);
  }
  terminalSessions.delete(serverId);

  if (activeSSHSessionId === serverId) {
    const remainingIds = Array.from(terminalSessions.keys());
    if (remainingIds.length > 0) {
      switchSSHTab(remainingIds[remainingIds.length - 1]);
    } else {
      activeSSHSessionId = null;
      const titleEl = document.getElementById('term-active-title');
      if (titleEl) titleEl.innerText = 'No active session';
    }
  }

  updateSSHTabsBar();
}

function updateSSHTabsBar() {
  const bar = document.getElementById('ssh-tabs-bar');
  const badge = document.getElementById('active-term-badge');
  if (!bar) return;

  if (terminalSessions.size > 0) {
    if (badge) badge.classList.remove('hidden');
  } else {
    if (badge) badge.classList.add('hidden');
  }

  let html = '';
  terminalSessions.forEach((sess, id) => {
    const isActive = id === activeSSHSessionId;
    const isOpen = sess.ws && sess.ws.readyState === WebSocket.OPEN;
    const dotClass = isOpen ? 'dot-online' : 'dot-offline';

    html += `
      <div onclick="switchSSHTab('${id}')" class="session-tab ${isActive ? 'active' : ''}">
        <span class="status-dot ${dotClass}"></span>
        <span class="truncate max-w-[140px]">${sess.name}</span>
        <span onclick="closeSSHSession('${id}', event)" class="session-tab-close" title="Close Session">✕</span>
      </div>
    `;
  });

  bar.innerHTML = html;
}

function focusActiveTerminal() {
  if (!activeSSHSessionId) return;
  const sess = terminalSessions.get(activeSSHSessionId);
  if (sess) {
    sess.term.focus();
    sess.containerEl.classList.add('terminal-focused-ring');
    setTimeout(() => {
      sess.containerEl.classList.remove('terminal-focused-ring');
    }, 1500);
  }
}

function changeTerminalFontSize(sizeStr) {
  const sz = parseInt(sizeStr) || 13;
  terminalSessions.forEach(sess => {
    sess.term.options.fontSize = sz;
    if (sess.fitAddon) sess.fitAddon.fit();
  });
  focusActiveTerminal();
}

function changeTerminalColorTheme(themeName) {
  const themes = getTerminalColorThemes();
  if (!themes[themeName]) return;
  terminalSessions.forEach(sess => {
    sess.term.options.theme = themes[themeName];
  });
  focusActiveTerminal();
}

function getTerminalColorThemes() {
  return {
    oled: {
      background: '#000000',
      foreground: '#e1e7ef',
      cursor: '#4b96ff',
      selectionBackground: 'rgba(75, 150, 255, 0.3)',
      black: '#000000', red: '#ef4444', green: '#10b981', yellow: '#f59e0b',
      blue: '#4b96ff', magenta: '#c084fc', cyan: '#22d3ee', white: '#e1e7ef'
    },
    warp: {
      background: '#0a0e17',
      foreground: '#e2e8f0',
      cursor: '#38bdf8',
      selectionBackground: 'rgba(56, 189, 248, 0.25)',
      black: '#0a0e17', red: '#f87171', green: '#4ade80', yellow: '#facc15',
      blue: '#38bdf8', magenta: '#e879f9', cyan: '#2dd4bf', white: '#f1f5f9'
    },
    matrix: {
      background: '#020a04',
      foreground: '#22c55e',
      cursor: '#4ade80',
      selectionBackground: 'rgba(34, 197, 94, 0.25)',
      black: '#020a04', red: '#ef4444', green: '#22c55e', yellow: '#eab308',
      blue: '#3b82f6', magenta: '#a855f7', cyan: '#14b8a6', white: '#dcfce7'
    },
    ubuntu: {
      background: '#2c001e',
      foreground: '#ffffff',
      cursor: '#e95420',
      selectionBackground: 'rgba(233, 84, 32, 0.35)',
      black: '#2c001e', red: '#ef4444', green: '#10b981', yellow: '#f59e0b',
      blue: '#3b82f6', magenta: '#c084fc', cyan: '#22d3ee', white: '#ffffff'
    },
    cyberpunk: {
      background: '#0f051d',
      foreground: '#facc15',
      cursor: '#f43f5e',
      selectionBackground: 'rgba(244, 63, 94, 0.35)',
      black: '#0f051d', red: '#f43f5e', green: '#10b981', yellow: '#facc15',
      blue: '#38bdf8', magenta: '#d946ef', cyan: '#06b6d4', white: '#fef08a'
    }
  };
}

function toggleTerminalFullscreen() {
  const header = document.querySelector('header.ent-header');
  const sidebar = document.getElementById('main-sidebar');
  const btn = document.getElementById('term-fullscreen-btn');

  isTerminalFullscreen = !isTerminalFullscreen;
  if (isTerminalFullscreen) {
    if (header) header.style.display = 'none';
    if (sidebar) sidebar.style.display = 'none';
    if (btn) btn.innerText = '⤓';
  } else {
    if (header) header.style.display = '';
    if (sidebar) sidebar.style.display = '';
    if (btn) btn.innerText = '⛶';
  }

  setTimeout(() => {
    if (activeSSHSessionId) {
      const sess = terminalSessions.get(activeSSHSessionId);
      if (sess && sess.fitAddon) sess.fitAddon.fit();
    }
    focusActiveTerminal();
  }, 100);
}

function terminalCopy() {
  if (!activeSSHSessionId) return;
  const sess = terminalSessions.get(activeSSHSessionId);
  if (!sess) return;
  const selection = sess.term.getSelection();
  if (selection) {
    navigator.clipboard.writeText(selection);
    alert('Selected text copied to clipboard!');
    sess.term.focus();
  } else {
    alert('No text selected in terminal.');
  }
}

async function terminalPaste() {
  if (!activeSSHSessionId) return;
  const sess = terminalSessions.get(activeSSHSessionId);
  if (!sess || !sess.ws || sess.ws.readyState !== WebSocket.OPEN) {
    alert('Terminal session is not connected.');
    return;
  }
  try {
    const text = await navigator.clipboard.readText();
    if (text) {
      // Chunk large pastes in 1KB blocks with 2ms backpressure delays
      const chunkSize = 1024;
      for (let i = 0; i < text.length; i += chunkSize) {
        const chunk = text.slice(i, i + chunkSize);
        if (sess.ws && sess.ws.readyState === WebSocket.OPEN) {
          sess.ws.send(JSON.stringify({ type: 'input', data: chunk }));
        }
        if (i + chunkSize < text.length) {
          await new Promise(r => setTimeout(r, 2));
        }
      }
      sess.term.focus();
    }
  } catch (err) {
    alert('Could not read from clipboard.');
  }
}

function terminalClear() {
  if (!activeSSHSessionId) return;
  const sess = terminalSessions.get(activeSSHSessionId);
  if (sess) {
    sess.term.clear();
    sess.term.focus();
  }
}

function reconnectActiveSSH() {
  if (!activeSSHSessionId) return;
  const id = activeSSHSessionId;
  closeSSHSession(id);
  openNewSSHConnection(id);
}

// ============================================================================
//  MULTI-SESSION SFTP FILE EXPLORER ENGINE (With Full Path Navigation)
// ============================================================================

function openSFTPForServer(id) {
  switchTab('sftp');
  switchSFTPServer(id);
}

function switchSFTPServer(serverId) {
  if (!serverId) return;
  currentSFTPServerId = serverId;

  const sel = document.getElementById('sftp-server-select');
  if (sel) sel.value = serverId;

  // Restore previous directory for this server, or default to "."
  let dirPath = '.';
  if (sftpSessions.has(serverId)) {
    dirPath = sftpSessions.get(serverId).currentPath || '.';
  }
  loadSFTPPath(dirPath);
}

async function loadSFTPPath(dirPath = '.') {
  if (!currentSFTPServerId) return;
  const id = currentSFTPServerId;

  const tbody = document.getElementById('sftp-table-body');
  if (tbody) {
    tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-[#8b95a5]">Loading remote directory...</td></tr>`;
  }

  try {
    const res = await fetch(`/api/sftp/list?id=${encodeURIComponent(id)}&path=${encodeURIComponent(dirPath)}`);
    const data = await res.json();
    if (!res.ok) {
      if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-[#ef4444]">Error: ${data.error || 'Path error'}</td></tr>`;
      return;
    }

    // Save state in sftpSessions map
    sftpSessions.set(id, {
      currentPath: data.current_path,
      parentPath: data.parent_path
    });

    renderBreadcrumb(data.current_path);
    const addressInput = document.getElementById('sftp-address-bar');
    if (addressInput) addressInput.value = data.current_path;

    if (!data.files || data.files.length === 0) {
      if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-[#8b95a5]">Directory is empty.</td></tr>`;
      return;
    }

    if (tbody) {
      tbody.innerHTML = data.files.map(fi => {
        const icon = fi.is_dir ? 
          `<span class="text-[#f59e0b]">📁</span>` : 
          `<span class="text-[#4b96ff]">📄</span>`;

        const clickAction = fi.is_dir ? 
          `onclick="loadSFTPPath('${fi.path.replace(/\\/g, '/')}')"` : 
          `onclick="openCodeModal('${fi.path.replace(/\\/g, '/')}', '${fi.name}')"`;

        return `
          <tr class="sftp-row">
            <td class="font-mono cursor-pointer" ${clickAction}>
              <div class="flex items-center gap-2">
                ${icon}
                <span class="hover:underline font-medium">${fi.name}</span>
              </div>
            </td>
            <td class="font-mono text-xs text-[#8b95a5]">${fi.size_str}</td>
            <td class="font-mono text-xs text-[#8b95a5]">${fi.permissions}</td>
            <td class="font-mono text-xs text-[#8b95a5]">${fi.mod_time}</td>
            <td class="text-right">
              <div class="flex items-center gap-1.5 justify-end">
                ${!fi.is_dir ? `
                  <a href="/api/sftp/download?id=${encodeURIComponent(id)}&path=${encodeURIComponent(fi.path)}" 
                     class="btn-default text-xs !px-2 !py-1" download>Download</a>
                  <button onclick="openCodeModal('${fi.path.replace(/\\/g, '/')}', '${fi.name}')" class="btn-default text-xs !px-2 !py-1">Edit</button>
                ` : ''}
                <button onclick="renameSFTP('${fi.path.replace(/\\/g, '/')}', '${fi.name}')" class="btn-default text-xs !px-2 !py-1">Rename</button>
                <button onclick="deleteSFTP('${fi.path.replace(/\\/g, '/')}')" class="btn-default text-xs !px-2 !py-1 text-[#ef4444]">Delete</button>
              </div>
            </td>
          </tr>
        `;
      }).join('');
    }
  } catch (err) {
    if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-[#ef4444]">SFTP Connection failed</td></tr>`;
  }
}

function sftpGoParent() {
  if (!currentSFTPServerId) return;
  const sess = sftpSessions.get(currentSFTPServerId);
  if (sess && sess.parentPath) {
    loadSFTPPath(sess.parentPath);
  } else {
    loadSFTPPath('..');
  }
}

function sftpGoHome() {
  if (!currentSFTPServerId) return;
  loadSFTPPath('.');
}

function sftpGoRoot() {
  if (!currentSFTPServerId) return;
  loadSFTPPath('/');
}

function sftpGoToAddress(event) {
  event.preventDefault();
  if (!currentSFTPServerId) return;
  const addressInput = document.getElementById('sftp-address-bar');
  if (addressInput && addressInput.value) {
    loadSFTPPath(addressInput.value.trim());
  }
}

function renderBreadcrumb(dirPath) {
  const bc = document.getElementById('sftp-breadcrumb');
  if (!bc) return;

  const parts = dirPath === '/' ? [''] : dirPath.split('/').filter(Boolean);
  let accumulated = '';

  const html = `<button onclick="loadSFTPPath('/')" class="hover:underline font-bold text-[#4b96ff]">root</button>` + parts.map(part => {
    accumulated += '/' + part;
    const p = accumulated;
    return `
      <span>/</span>
      <button onclick="loadSFTPPath('${p}')" class="hover:underline">${part}</button>
    `;
  }).join('');

  bc.innerHTML = html;
}

function refreshSFTP() {
  if (!currentSFTPServerId) return;
  const sess = sftpSessions.get(currentSFTPServerId);
  const targetPath = sess ? (sess.currentPath || '.') : '.';
  loadSFTPPath(targetPath);
}

async function handleSFTPUpload(input) {
  const file = input.files?.[0];
  if (!file || !currentSFTPServerId) return;

  const sess = sftpSessions.get(currentSFTPServerId);
  const targetPath = sess ? (sess.currentPath || '.') : '.';

  const form = new FormData();
  form.append('file', file);

  try {
    const res = await fetch(`/api/sftp/upload?id=${encodeURIComponent(currentSFTPServerId)}&path=${encodeURIComponent(targetPath)}`, {
      method: 'POST',
      body: form
    });
    const data = await res.json();
    if (res.ok) {
      alert('File uploaded successfully.');
      refreshSFTP();
    } else {
      alert(data.error || 'Upload error');
    }
  } catch (err) {
    alert('Server communication error.');
  } finally {
    input.value = '';
  }
}

async function promptMkdir() {
  if (!currentSFTPServerId) {
    alert('Please select a server first.');
    return;
  }
  const folderName = prompt('New folder name:', 'new_directory');
  if (!folderName) return;

  const sess = sftpSessions.get(currentSFTPServerId);
  const targetPath = sess ? (sess.currentPath || '.') : '.';
  const remotePath = `${targetPath === '/' ? '' : targetPath + '/'}${folderName}`;

  const res = await fetch('/api/sftp/mkdir', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ server_id: currentSFTPServerId, path: remotePath })
  });
  if (res.ok) {
    refreshSFTP();
  } else {
    alert('Error creating folder');
  }
}

async function promptTouch() {
  if (!currentSFTPServerId) {
    alert('Please select a server first.');
    return;
  }
  const fileName = prompt('New file name:', 'new_file.txt');
  if (!fileName) return;

  const sess = sftpSessions.get(currentSFTPServerId);
  const targetPath = sess ? (sess.currentPath || '.') : '.';
  const remotePath = `${targetPath === '/' ? '' : targetPath + '/'}${fileName}`;

  const res = await fetch('/api/sftp/touch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ server_id: currentSFTPServerId, path: remotePath })
  });
  if (res.ok) {
    refreshSFTP();
  } else {
    alert('Error creating file');
  }
}

async function deleteSFTP(path) {
  if (!confirm(`Delete "${path}"?`)) return;
  const res = await fetch('/api/sftp/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ server_id: currentSFTPServerId, path: path })
  });
  if (res.ok) {
    refreshSFTP();
  } else {
    alert('Error deleting item');
  }
}

async function renameSFTP(oldPath, oldName) {
  const newName = prompt('New name:', oldName);
  if (!newName || newName === oldName) return;

  const parent = oldPath.substring(0, oldPath.lastIndexOf('/'));
  const newPath = parent ? `${parent}/${newName}` : newName;

  const res = await fetch('/api/sftp/rename', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ server_id: currentSFTPServerId, old_path: oldPath, new_path: newPath })
  });
  if (res.ok) {
    refreshSFTP();
  } else {
    alert('Error renaming item');
  }
}

// ---- Code Editor Modal ----
async function openCodeModal(path, name) {
  if (!currentSFTPServerId) return;
  const modal = document.getElementById('code-modal');
  const title = document.getElementById('code-modal-title');
  const pathInput = document.getElementById('code-file-path');
  const editor = document.getElementById('code-file-editor');

  title.innerText = `Editing: ${name} (${path})`;
  pathInput.value = path;
  editor.value = '--- Loading remote file content ---';
  modal.classList.remove('hidden');

  try {
    const res = await fetch(`/api/sftp/read?id=${encodeURIComponent(currentSFTPServerId)}&path=${encodeURIComponent(path)}`);
    const data = await res.json();
    if (res.ok) {
      editor.value = data.content;
    } else {
      editor.value = `Error reading file: ${data.error}`;
    }
  } catch (err) {
    editor.value = 'Server communication error.';
  }
}

function closeCodeModal() {
  document.getElementById('code-modal').classList.add('hidden');
}

async function saveCodeEditor() {
  const path = document.getElementById('code-file-path').value;
  const content = document.getElementById('code-file-editor').value;

  try {
    const res = await fetch('/api/sftp/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        server_id: currentSFTPServerId,
        path: path,
        content: content
      })
    });
    if (res.ok) {
      alert('File saved successfully.');
      closeCodeModal();
      refreshSFTP();
    } else {
      alert('Error saving file');
    }
  } catch (err) {
    alert('Error sending data to server');
  }
}

// ---- DevOps Handlers ----

async function loadServices() {
  const select = document.getElementById('services-server-select');
  const id = select?.value;
  const tbody = document.getElementById('services-table-body');
  if (!id || !tbody) return;

  tbody.innerHTML = `<tr><td colspan="6" class="py-6 text-center text-[#8b95a5]">Loading systemd units from server...</td></tr>`;
  try {
    const res = await fetch(`/api/services?id=${encodeURIComponent(id)}`);
    const data = await res.json();
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="6" class="py-6 text-center text-[#ef4444]">Error: ${data.error}</td></tr>`;
      return;
    }
    tbody.innerHTML = data.map(srv => `
      <tr class="sftp-row">
        <td class="font-mono font-medium text-white">${srv.name}</td>
        <td class="font-mono text-xs text-[#8b95a5]">${srv.load}</td>
        <td class="font-mono text-xs ${srv.active === 'active' ? 'text-[#10b981]' : 'text-[#ef4444]'}">${srv.active}</td>
        <td class="font-mono text-xs text-[#8b95a5]">${srv.sub}</td>
        <td class="text-xs text-[#8b95a5]">${srv.description}</td>
        <td class="text-right">
          <div class="flex items-center gap-1.5 justify-end">
            <button onclick="serviceAction('${id}', 'restart', '${srv.name}')" class="btn-default text-xs !px-2 !py-1">Restart</button>
            <button onclick="serviceAction('${id}', 'stop', '${srv.name}')" class="btn-default text-xs !px-2 !py-1 text-[#ef4444]">Stop</button>
            <button onclick="serviceAction('${id}', 'start', '${srv.name}')" class="btn-default text-xs !px-2 !py-1 text-[#10b981]">Start</button>
          </div>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" class="py-6 text-center text-[#ef4444]">Connection error</td></tr>`;
  }
}

async function serviceAction(serverID, action, serviceName) {
  try {
    const res = await fetch('/api/services/action', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server_id: serverID, action: action, service: serviceName })
    });
    const data = await res.json();
    alert(`Service Action [${action.toUpperCase()} ${serviceName}]:\n` + (data.output || 'Command sent successfully.'));
    loadServices();
  } catch (err) {
    alert('Error executing service action.');
  }
}

async function loadContainers() {
  const select = document.getElementById('docker-server-select');
  const id = select?.value;
  const tbody = document.getElementById('docker-table-body');
  if (!id || !tbody) return;

  tbody.innerHTML = `<tr><td colspan="6" class="py-6 text-center text-[#8b95a5]">Loading Docker containers from server...</td></tr>`;
  try {
    const res = await fetch(`/api/docker/containers?id=${encodeURIComponent(id)}`);
    const data = await res.json();
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="6" class="py-6 text-center text-[#ef4444]">Error: ${data.error}</td></tr>`;
      return;
    }
    if (data.length === 0) {
      tbody.innerHTML = `<tr><td colspan="6" class="py-6 text-center text-[#8b95a5]">No Docker containers found on this server.</td></tr>`;
      return;
    }
    tbody.innerHTML = data.map(c => `
      <tr class="sftp-row">
        <td class="font-mono font-medium text-white">${c.id}</td>
        <td class="font-mono text-xs text-[#4b96ff]">${c.names}</td>
        <td class="font-mono text-xs text-[#8b95a5]">${c.image}</td>
        <td class="font-mono text-xs ${c.state === 'running' ? 'text-[#10b981]' : 'text-[#ef4444]'}">${c.status}</td>
        <td class="font-mono text-xs text-[#8b95a5]">${c.ports}</td>
        <td class="text-right">
          <div class="flex items-center gap-1.5 justify-end">
            <button onclick="containerAction('${id}', 'logs', '${c.id}')" class="btn-default text-xs !px-2 !py-1">Logs</button>
            <button onclick="containerAction('${id}', 'restart', '${c.id}')" class="btn-default text-xs !px-2 !py-1">Restart</button>
            <button onclick="containerAction('${id}', 'stop', '${c.id}')" class="btn-default text-xs !px-2 !py-1 text-[#ef4444]">Stop</button>
          </div>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="6" class="py-6 text-center text-[#ef4444]">Connection error</td></tr>`;
  }
}

async function containerAction(serverID, action, containerID) {
  try {
    const res = await fetch('/api/docker/action', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server_id: serverID, action: action, container_id: containerID })
    });
    const data = await res.json();
    alert(`Docker Action [${action.toUpperCase()} ${containerID}]:\n` + (data.output || 'Command sent successfully.'));
    if (action !== 'logs') loadContainers();
  } catch (err) {
    alert('Error executing docker action.');
  }
}

async function loadPorts() {
  const select = document.getElementById('ports-server-select');
  const id = select?.value;
  const tbody = document.getElementById('ports-table-body');
  if (!id || !tbody) return;

  tbody.innerHTML = `<tr><td colspan="4" class="py-6 text-center text-[#8b95a5]">Scanning listening TCP/UDP network ports...</td></tr>`;
  try {
    const res = await fetch(`/api/ports?id=${encodeURIComponent(id)}`);
    const data = await res.json();
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="4" class="py-6 text-center text-[#ef4444]">Error: ${data.error}</td></tr>`;
      return;
    }
    tbody.innerHTML = data.map(p => `
      <tr class="sftp-row">
        <td class="font-mono font-medium text-[#4b96ff]">${p.proto}</td>
        <td class="font-mono text-xs text-[#10b981]">${p.state}</td>
        <td class="font-mono text-xs text-white">${p.local_addr}</td>
        <td class="font-mono text-xs text-[#8b95a5]">${p.foreign_addr}</td>
      </tr>
    `).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="py-6 text-center text-[#ef4444]">Connection error</td></tr>`;
  }
}

// 4. SSH Key Generator
async function generateKeyPair() {
  const keyType = document.getElementById('keygen-type').value || 'rsa';
  try {
    const res = await fetch('/api/ssh/keygen', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key_type: keyType })
    });
    const data = await res.json();
    if (res.ok) {
      document.getElementById('gen-pub-key').value = data.public_key;
      document.getElementById('gen-priv-key').value = data.private_key;
      document.getElementById('keygen-output').classList.remove('hidden');
    } else {
      alert('Error generating key: ' + data.error);
    }
  } catch (err) {
    alert('Network error generating SSH key.');
  }
}

function copyInput(id) {
  const el = document.getElementById(id);
  if (el && el.value) {
    navigator.clipboard.writeText(el.value);
    alert('Public key copied to clipboard!');
  }
}

// 5. Remote ~/.ssh/authorized_keys
async function loadAuthorizedKeys() {
  const id = document.getElementById('authkeys-server-select')?.value;
  const textarea = document.getElementById('remote-authkeys-output');
  if (!id || !textarea) return;

  textarea.value = '--- Loading authorized keys from server ---';
  try {
    const res = await fetch(`/api/ssh/authorized_keys?id=${encodeURIComponent(id)}`);
    const data = await res.json();
    if (res.ok) {
      textarea.value = data.keys || 'No authorized keys found.';
    } else {
      textarea.value = `Error: ${data.error}`;
    }
  } catch (err) {
    textarea.value = 'Network connection error.';
  }
}

async function addRemoteAuthorizedKey() {
  const id = document.getElementById('authkeys-server-select')?.value;
  const pubKey = document.getElementById('add-pub-key-input')?.value;
  if (!id || !pubKey) {
    alert('Please select a server and enter a valid SSH Public Key.');
    return;
  }
  try {
    const res = await fetch('/api/ssh/authorized_keys/add', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server_id: id, public_key: pubKey })
    });
    const data = await res.json();
    if (res.ok) {
      alert('Public key authorized successfully on target server!');
      document.getElementById('add-pub-key-input').value = '';
      loadAuthorizedKeys();
    } else {
      alert('Error authorizing key: ' + data.error);
    }
  } catch (err) {
    alert('Network error.');
  }
}

// 6. Multi-Server Cluster Command Broadcast
function renderClusterCheckboxes() {
  const container = document.getElementById('cluster-server-checkboxes');
  if (!container) return;

  if (servers.length === 0) {
    container.innerHTML = `<p class="text-xs text-[#8b95a5]">No servers configured yet.</p>`;
    return;
  }

  container.innerHTML = servers.map(s => `
    <label class="ent-panel px-3 py-1.5 flex items-center gap-2 cursor-pointer text-xs font-semibold">
      <input type="checkbox" name="cluster-server-chk" value="${s.id}" checked class="rounded">
      <span>${s.name} (${s.host})</span>
    </label>
  `).join('');
}

function selectAllServersCluster() {
  document.querySelectorAll('input[name="cluster-server-chk"]').forEach(chk => {
    chk.checked = true;
  });
}

async function runClusterCommand() {
  const selectedIDs = [];
  document.querySelectorAll('input[name="cluster-server-chk"]:checked').forEach(chk => {
    selectedIDs.push(chk.value);
  });

  if (selectedIDs.length === 0) {
    alert('Please select at least one server.');
    return;
  }

  const cmd = document.getElementById('cluster-cmd-input')?.value;
  if (!cmd) {
    alert('Please enter a Linux command to broadcast.');
    return;
  }

  const grid = document.getElementById('cluster-results-grid');
  grid.innerHTML = `<div class="col-span-full ent-panel p-6 text-center text-[#8b95a5]">Broadcasting command across ${selectedIDs.length} servers concurrently using Goroutines...</div>`;

  try {
    const res = await fetch('/api/cluster/exec', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server_ids: selectedIDs, command: cmd })
    });
    const data = await res.json();
    grid.innerHTML = data.map(r => `
      <div class="ent-panel p-4 space-y-2">
        <div class="flex items-center justify-between">
          <span class="text-xs font-bold text-white">${r.server_name}</span>
          <span class="text-[10px] font-mono text-[#8b95a5]">${r.duration_ms} ms</span>
        </div>
        <pre class="bg-[#05070a] p-3 rounded font-mono text-xs ${r.error ? 'text-[#ef4444]' : 'text-[#10b981]'} min-h-[100px] max-h-[200px] overflow-auto border border-[#212836]">${r.output || r.error || '(no output)'}</pre>
      </div>
    `).join('');
  } catch (err) {
    grid.innerHTML = `<div class="col-span-full ent-panel p-6 text-center text-[#ef4444]">Network error broadcasting command.</div>`;
  }
}

// ---- Backup Export & Import ----
function exportBackup() {
  window.location.href = '/api/export';
}

async function importBackup(input) {
  const file = input.files?.[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = async (e) => {
    try {
      const json = JSON.parse(e.target.result);
      const res = await fetch('/api/import', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(json)
      });
      if (res.ok) {
        alert('Configuration backup restored successfully!');
        loadServers();
        loadSnippets();
      } else {
        alert('Failed to import backup JSON.');
      }
    } catch (err) {
      alert('Invalid JSON file format.');
    }
  };
  reader.readAsText(file);
}

// ---- Load & Render Snippets ----
async function loadSnippets() {
  try {
    const res = await fetch('/api/snippets');
    if (!res.ok) return;
    snippets = await res.json();
    renderSnippets();
  } catch (err) {
    console.error('Error loading snippets:', err);
  }
}

function renderSnippets() {
  const list = document.getElementById('snippets-list');
  if (!list) return;

  if (snippets.length === 0) {
    list.innerHTML = `<div class="col-span-full ent-panel p-6 text-center text-[#8b95a5]">No snippets found.</div>`;
    return;
  }

  const serverOpts = `<option value="">Run on server...</option>` + servers.map(s => `
    <option value="${s.id}">${s.name}</option>
  `).join('');

  list.innerHTML = snippets.map(sn => {
    const tagsHTML = (sn.tags || []).map(t => `<span class="tag-badge">#${t}</span>`).join(' ');
    return `
      <div class="ent-card p-4 flex flex-col justify-between space-y-3">
        <div>
          <div class="flex items-start justify-between">
            <h3 class="text-sm font-bold text-white">${sn.title}</h3>
            <button onclick="deleteSnippet('${sn.id}')" class="text-[#ef4444] hover:underline text-xs">Delete</button>
          </div>
          <p class="text-xs text-[#8b95a5] mt-1">${sn.description || ''}</p>
          <pre class="mt-2 bg-[#05070a] p-2.5 rounded font-mono text-xs text-[#4b96ff] overflow-x-auto border border-[#212836]" dir="ltr">${sn.command}</pre>
          <div class="mt-2 flex flex-wrap gap-1">${tagsHTML}</div>
        </div>

        <div class="pt-2 border-t border-[#212836] flex items-center justify-between">
          <select id="run-srv-${sn.id}" class="ent-input text-xs">
            ${serverOpts}
          </select>
          <button onclick="executeSnippet('${sn.id}')" class="btn-primary text-xs" data-i18n="btn_run_srv">
            Run on Server
          </button>
        </div>
      </div>
    `;
  }).join('');
}

async function executeSnippet(snippetId) {
  const sn = snippets.find(s => s.id === snippetId);
  if (!sn) return;
  const select = document.getElementById(`run-srv-${snippetId}`);
  const srvId = select?.value;
  if (!srvId) {
    alert('Please select a server.');
    return;
  }

  const srv = servers.find(s => s.id === srvId);
  const outEl = document.getElementById('snippet-output');
  const statusEl = document.getElementById('cmd-run-status');

  statusEl.innerText = `Running "${sn.title}" on ${srv ? srv.name : srvId}...`;
  outEl.innerText = `--- EXEC: ${sn.command} ---\r\nConnecting...`;

  try {
    const res = await fetch('/api/command', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server_id: srvId, command: sn.command })
    });
    const data = await res.json();
    if (res.ok) {
      statusEl.innerText = `Executed (${new Date().toLocaleTimeString()})`;
      outEl.innerText = `=== OUTPUT (${srv ? srv.name : srvId}) ===\r\n\r\n` + (data.output || '(no output)') + (data.error ? `\r\n\r\n--- ERROR ---\r\n${data.error}` : '');
    } else {
      statusEl.innerText = 'Execution Error';
      outEl.innerText = data.error;
    }
  } catch (err) {
    statusEl.innerText = 'Network error';
    outEl.innerText = 'Connection to server failed.';
  }
}

// ---- Server Monitor ----
function openMonitorForServer(id) {
  const select = document.getElementById('monitor-server-select');
  if (select) select.value = id;
  switchTab('monitor');
  loadServerStats();
}

async function loadServerStats() {
  const select = document.getElementById('monitor-server-select');
  const id = select?.value;
  if (!id) return;

  const cpuText = document.getElementById('cpu-val-text');
  const ramText = document.getElementById('ram-val-text');
  const diskText = document.getElementById('disk-val-text');
  const cpuBar = document.getElementById('cpu-bar');
  const ramBar = document.getElementById('ram-bar');
  const diskBar = document.getElementById('disk-bar');
  const cpuDetail = document.getElementById('cpu-load-detail');
  const ramDetail = document.getElementById('ram-detail');
  const diskDetail = document.getElementById('disk-detail');
  const osText = document.getElementById('os-name-text');
  const uptimeText = document.getElementById('uptime-text');

  if (cpuText) cpuText.innerText = 'Querying...';

  try {
    const res = await fetch(`/api/monitor?id=${encodeURIComponent(id)}`);
    const data = await res.json();

    if (!res.ok || data.error) {
      if (cpuText) cpuText.innerText = 'Error';
      if (osText) osText.innerText = data.error || 'Connection error';
      return;
    }

    if (cpuText) cpuText.innerText = `${data.cpu_percent || 0}%`;
    if (ramText) ramText.innerText = `${data.ram_percent || 0}%`;
    if (diskText) diskText.innerText = `${data.disk_percent || 0}%`;

    if (cpuBar) cpuBar.style.width = `${Math.min(data.cpu_percent || 0, 100)}%`;
    if (ramBar) ramBar.style.width = `${Math.min(data.ram_percent || 0, 100)}%`;
    if (diskBar) diskBar.style.width = `${Math.min(data.disk_percent || 0, 100)}%`;

    if (cpuDetail) cpuDetail.innerText = `Load Avg (1/5/15): ${data.load_avg || '-'}`;
    if (ramDetail) ramDetail.innerText = `${data.ram_used_mb || 0} MB / ${data.ram_total_mb || 0} MB`;
    if (diskDetail) diskDetail.innerText = `${data.disk_used_gb || 0} GB / ${data.disk_total_gb || 0} GB`;

    if (osText) osText.innerText = `${data.hostname} — ${data.os_info}`;
    if (uptimeText) uptimeText.innerText = data.uptime || '-';
  } catch (err) {
    if (cpuText) cpuText.innerText = 'Error';
  }
}

// ---- Ping Servers ----
async function pingAllServers() {
  try {
    await fetch('/api/servers/ping-all', { method: 'POST' });
    loadServers(true);
  } catch (err) {
    console.error('Error pinging all:', err);
  }
}

// ---- Modals & CRUD ----
function openServerModal() {
  document.getElementById('srv-id').value = '';
  document.getElementById('srv-name').value = '';
  document.getElementById('srv-host').value = '';
  document.getElementById('srv-port').value = '22';
  document.getElementById('srv-username').value = 'root';
  document.getElementById('srv-password').value = '';
  document.getElementById('srv-private-key').value = '';
  document.getElementById('srv-passphrase').value = '';
  document.getElementById('srv-tags').value = '';
  document.getElementById('srv-notes').value = '';
  document.getElementById('server-modal-title').innerText = 'Add New Server';
  document.getElementById('server-modal').classList.remove('hidden');
}

function closeServerModal() {
  document.getElementById('server-modal').classList.add('hidden');
}

function toggleAuthFields() {
  const type = document.getElementById('srv-auth-type').value;
  const passDiv = document.getElementById('field-password');
  const keyDiv = document.getElementById('field-key');
  if (type === 'key') {
    passDiv.classList.add('hidden');
    keyDiv.classList.remove('hidden');
  } else {
    passDiv.classList.remove('hidden');
    keyDiv.classList.add('hidden');
  }
}

function editServer(id) {
  const s = servers.find(x => x.id === id);
  if (!s) return;
  document.getElementById('srv-id').value = s.id;
  document.getElementById('srv-name').value = s.name;
  document.getElementById('srv-group').value = s.group;
  document.getElementById('srv-host').value = s.host;
  document.getElementById('srv-port').value = s.port;
  document.getElementById('srv-username').value = s.username;
  document.getElementById('srv-auth-type').value = s.auth_type || 'password';
  document.getElementById('srv-password').value = s.password || '';
  document.getElementById('srv-private-key').value = s.private_key || '';
  document.getElementById('srv-passphrase').value = s.passphrase || '';
  document.getElementById('srv-tags').value = (s.tags || []).join(', ');
  document.getElementById('srv-notes').value = s.notes || '';
  document.getElementById('server-modal-title').innerText = `Edit Server "${s.name}"`;
  toggleAuthFields();
  document.getElementById('server-modal').classList.remove('hidden');
}

async function saveServerForm(e) {
  e.preventDefault();
  const id = document.getElementById('srv-id').value;
  const name = document.getElementById('srv-name').value;
  const group = document.getElementById('srv-group').value;
  const host = document.getElementById('srv-host').value;
  const port = parseInt(document.getElementById('srv-port').value) || 22;
  const username = document.getElementById('srv-username').value;
  const auth_type = document.getElementById('srv-auth-type').value;
  const password = document.getElementById('srv-password').value;
  const private_key = document.getElementById('srv-private-key').value;
  const passphrase = document.getElementById('srv-passphrase').value;
  const tagsRaw = document.getElementById('srv-tags').value;
  const notes = document.getElementById('srv-notes').value;

  const tags = tagsRaw.split(',').map(t => t.trim()).filter(Boolean);

  const payload = {
    id, name, group, host, port, username, auth_type,
    password, private_key, passphrase, tags, notes
  };

  try {
    const res = await fetch('/api/servers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (res.ok) {
      closeServerModal();
      loadServers();
    } else {
      const data = await res.json();
      alert(`Error saving server: ${data.error}`);
    }
  } catch (err) {
    alert('Server communication error.');
  }
}

async function deleteServer(id) {
  if (!confirm('Are you sure you want to delete this server?')) return;
  try {
    await fetch(`/api/servers?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
    loadServers();
  } catch (err) {
    alert('Error deleting server');
  }
}

// ---- Snippet Modal ----
function openSnippetModal() {
  document.getElementById('sn-title').value = '';
  document.getElementById('sn-cmd').value = '';
  document.getElementById('sn-desc').value = '';
  document.getElementById('sn-tags').value = '';
  document.getElementById('snippet-modal').classList.remove('hidden');
}

function closeSnippetModal() {
  document.getElementById('snippet-modal').classList.add('hidden');
}

async function saveSnippetForm(e) {
  e.preventDefault();
  const title = document.getElementById('sn-title').value;
  const command = document.getElementById('sn-cmd').value;
  const description = document.getElementById('sn-desc').value;
  const tagsRaw = document.getElementById('sn-tags').value;
  const tags = tagsRaw.split(',').map(t => t.trim()).filter(Boolean);

  try {
    const res = await fetch('/api/snippets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, command, description, tags })
    });
    if (res.ok) {
      closeSnippetModal();
      loadSnippets();
    } else {
      alert('Error saving snippet');
    }
  } catch (err) {
    alert('Network error');
  }
}

async function deleteSnippet(id) {
  if (!confirm('Are you sure you want to delete this snippet?')) return;
  try {
    await fetch(`/api/snippets?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
    loadSnippets();
  } catch (err) {
    alert('Error deleting snippet');
  }
}
