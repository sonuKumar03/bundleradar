let bundleData = null;
let treemap = null;
let currentPackages = [];
let selectedPkgName = '';
let currentFilter = 'all';

// Multi-version & Drift state
let buildHistory = [];
let baselineId = '';
let activeBuildId = 'latest';
let activeTab = 'diagnosis';
let currentDiffData = null;
let driftFilter = 'all';

// Tab Navigation
function switchTab(tabId) {
  activeTab = tabId;
  document.querySelectorAll('.tab-btn').forEach(btn => {
    if (btn.dataset.tab === tabId) {
      btn.className = 'tab-btn px-3 py-1.5 rounded border text-xs flex items-center space-x-1.5 tab-active transition-all';
    } else {
      btn.className = 'tab-btn px-3 py-1.5 rounded border text-xs flex items-center space-x-1.5 tab-inactive transition-all';
    }
  });

  document.querySelectorAll('.view-panel').forEach(panel => {
    panel.classList.add('hidden');
  });

  const activePanel = document.getElementById(`view-${tabId}`);
  if (activePanel) {
    activePanel.classList.remove('hidden');
  }

  if (tabId === 'treemap' && treemap) {
    setTimeout(() => treemap.resizeAndDraw(), 50);
  } else if (tabId === 'compare') {
    loadAndRenderDiff();
  } else if (tabId === 'side-by-side') {
    loadAndRenderSideBySide();
  }

  if (history.replaceState) {
    history.replaceState(null, '', `#${tabId}`);
  }
}

async function init() {
  treemap = new TreemapEngine('treemap-canvas', 'tooltip');

  // Wire Treemap tile click to jump directly into the Ingress & Packages tab
  treemap.onSelect = (pkg) => {
    if (!pkg || !pkg.name) return;
    switchTab('packages');
    selectPackage(pkg.name);
  };

  // Wire Tab buttons
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      switchTab(btn.dataset.tab);
    });
  });

  // Wire Version Selector
  const versionSelect = document.getElementById('version-select');
  if (versionSelect) {
    versionSelect.addEventListener('change', async (e) => {
      activeBuildId = e.target.value;
      await loadBundle(activeBuildId);
    });
  }

  // Wire Pin Baseline Button
  const pinBaselineBtn = document.getElementById('btn-pin-baseline');
  if (pinBaselineBtn) {
    pinBaselineBtn.addEventListener('click', async () => {
      const targetId = activeBuildId === 'latest' && buildHistory.length > 0 
        ? buildHistory[buildHistory.length - 1].id 
        : activeBuildId;
      if (!targetId) return;

      try {
        const res = await fetch(`/api/baseline?id=${encodeURIComponent(targetId)}`, { method: 'POST' });
        if (res.ok) {
          showToast(`📌 Set ${targetId} as Baseline`, 'success');
          await loadBuildHistory();
          if (activeTab === 'compare') {
            loadAndRenderDiff();
          } else if (activeTab === 'side-by-side') {
            loadAndRenderSideBySide();
          }
        }
      } catch (err) {
        console.error('Failed to set baseline:', err);
      }
    });
  }

  // Wire Compare Selectors
  const compareTarget = document.getElementById('compare-target-select');
  const compareBase = document.getElementById('compare-base-select');
  if (compareTarget && compareBase) {
    compareTarget.addEventListener('change', () => loadAndRenderDiff());
    compareBase.addEventListener('change', () => loadAndRenderDiff());
  }

  // Wire Side-by-Side Selectors & Controls
  const sbsTarget = document.getElementById('sbs-target-select');
  const sbsBase = document.getElementById('sbs-base-select');
  if (sbsTarget && sbsBase) {
    sbsTarget.addEventListener('change', () => loadAndRenderSideBySide());
    sbsBase.addEventListener('change', () => loadAndRenderSideBySide());
  }

  document.querySelectorAll('.sbs-filter-chip').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.sbs-filter-chip').forEach(b => {
        b.className = 'sbs-filter-chip px-2.5 py-1 rounded text-[11px] font-mono border bg-[#0d1117] text-[#8b949e] border-[#30363d] hover:text-[#e6edf3]';
      });
      btn.className = 'sbs-filter-chip px-2.5 py-1 rounded text-[11px] font-mono border bg-[#21262d] text-[#e6edf3] border-[#58a6ff]';
      sbsFilter = btn.dataset.sbsFilter || 'all';
      renderSideBySideRows();
    });
  });

  const sbsSearch = document.getElementById('sbs-search-input');
  if (sbsSearch) {
    sbsSearch.addEventListener('input', (e) => {
      sbsSearchQuery = e.target.value.trim();
      renderSideBySideRows();
    });
  }

  const sbsDetailClose = document.getElementById('sbs-detail-close');
  if (sbsDetailClose) {
    sbsDetailClose.addEventListener('click', () => {
      document.getElementById('sbs-detail-panel')?.classList.add('hidden');
    });
  }

  // Wire Drift Filter Chips
  document.querySelectorAll('.drift-filter-chip').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.drift-filter-chip').forEach(b => {
        b.className = 'drift-filter-chip px-2.5 py-1 rounded bg-[#0d1117] text-[#8b949e] hover:text-[#e6edf3] border border-[#30363d]';
      });
      btn.className = 'drift-filter-chip px-2.5 py-1 rounded bg-[#21262d] text-[#58a6ff] border border-[#30363d]';
      driftFilter = btn.dataset.driftFilter || 'all';
      renderDriftTable(currentDiffData);
    });
  });

  // Filter chips in Packages Explorer
  document.querySelectorAll('.filter-chip').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.filter-chip').forEach(b => {
        b.className = 'filter-chip px-2 py-0.5 rounded bg-[#0d1117] text-[#8b949e] hover:text-[#e6edf3] border border-[#30363d]';
      });
      btn.className = 'filter-chip px-2 py-0.5 rounded bg-[#21262d] text-[#58a6ff] border border-[#30363d]';
      currentFilter = btn.dataset.filter || 'all';
      applyFilterAndSearch();
    });
  });

  // Keyboard shortcut: Cmd+K or / to search, Esc to clear
  window.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      const input = document.getElementById('search-input');
      if (input) input.focus();
    } else if (e.key === '/' && document.activeElement.tagName !== 'INPUT') {
      e.preventDefault();
      const input = document.getElementById('search-input');
      if (input) input.focus();
    } else if (e.key === 'Escape') {
      const input = document.getElementById('search-input');
      if (input && input.value) {
        input.value = '';
        document.getElementById('clear-search')?.classList.add('hidden');
        treemap.setHighlight('');
        applyFilterAndSearch();
      }
    }
  });

  // Search input and clear button
  const searchInput = document.getElementById('search-input');
  const clearBtn = document.getElementById('clear-search');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      const q = e.target.value;
      if (clearBtn) {
        if (q) clearBtn.classList.remove('hidden');
        else clearBtn.classList.add('hidden');
      }
      treemap.setHighlight(q);
      applyFilterAndSearch();
    });
  }

  if (clearBtn && searchInput) {
    clearBtn.addEventListener('click', () => {
      searchInput.value = '';
      clearBtn.classList.add('hidden');
      treemap.setHighlight('');
      applyFilterAndSearch();
    });
  }

  // Load initial bundle and history
  await loadBuildHistory();
  await loadBundle('latest');

  // Check URL hash for initial tab deep link
  const hashTab = window.location.hash.replace('#', '');
  if (['diagnosis', 'compare', 'side-by-side', 'packages', 'treemap'].includes(hashTab)) {
    switchTab(hashTab);
  }

  // Connect Server-Sent Events for Live Updates without Reloading
  setupLiveSync();
}

// ==========================================
// LIVE SYNC ENGINE (SSE - ZERO RELOAD)
// ==========================================
function setupLiveSync() {
  if (!window.EventSource) return;

  const es = new EventSource('/api/events');

  es.addEventListener('open', () => {
    const ind = document.getElementById('live-indicator');
    if (ind) {
      ind.classList.remove('opacity-50');
      ind.title = 'Live Sync connected';
    }
  });

  es.addEventListener('error', () => {
    const ind = document.getElementById('live-indicator');
    if (ind) {
      ind.classList.add('opacity-50');
      ind.title = 'Live Sync reconnecting...';
    }
  });

  es.addEventListener('build', async (e) => {
    try {
      const cp = JSON.parse(e.data);
      console.log('⚡ Live build update received:', cp);

      // Pulse animation on the Live Sync badge
      const ind = document.getElementById('live-indicator');
      if (ind) {
        ind.classList.add('ring-2', 'ring-[#3fb950]');
        setTimeout(() => ind.classList.remove('ring-2', 'ring-[#3fb950]'), 1200);
      }

      showToast(`⚡ ${cp.label || cp.id} parsed live! (${formatBytes(cp.initialBytes)} Initial JS)`, 'build');

      // Refresh build history
      await loadBuildHistory();

      // If user is currently looking at latest build, update in real-time
      if (activeBuildId === 'latest' || activeBuildId === cp.id) {
        await loadBundle(activeBuildId);
      }

      // If user is in Compare view, update diff live
      if (activeTab === 'compare') {
        loadAndRenderDiff();
      }
    } catch (err) {
      console.error('Failed to parse live build event:', err);
    }
  });
}

// ==========================================
// TOAST NOTIFICATIONS
// ==========================================
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = 'pointer-events-auto flex items-center space-x-2 px-4 py-2.5 rounded-lg border text-xs font-mono shadow-2xl transition-all duration-300 transform translate-y-2 opacity-0';

  if (type === 'build') {
    toast.className += ' bg-[#161b22] border-[#3fb950] text-[#e6edf3]';
  } else if (type === 'success') {
    toast.className += ' bg-[#161b22] border-[#58a6ff] text-[#e6edf3]';
  } else {
    toast.className += ' bg-[#161b22] border-[#30363d] text-[#c9d1d9]';
  }

  toast.innerHTML = `
    <span class="text-[#3fb950] font-bold">⚡</span>
    <span>${escapeHTML(message)}</span>
  `;

  container.appendChild(toast);

  // Trigger animation in
  setTimeout(() => {
    toast.classList.remove('translate-y-2', 'opacity-0');
  }, 10);

  // Auto dismiss after 4 seconds
  setTimeout(() => {
    toast.classList.add('translate-y-2', 'opacity-0');
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

// ==========================================
// BUILD HISTORY & CHECKPOINT MANAGEMENT
// ==========================================
async function loadBuildHistory() {
  try {
    const res = await fetch('/api/history');
    if (!res.ok) return;
    const data = await res.json();
    buildHistory = data.checkpoints || [];
    baselineId = data.baselineId || (buildHistory[0] ? buildHistory[0].id : '');

    updateVersionSelectDropdowns();
    updateCompareTabBadge();
    if (activeTab === 'compare') {
      loadAndRenderDiff();
    } else if (activeTab === 'side-by-side') {
      loadAndRenderSideBySide();
    }
  } catch (err) {
    console.error('Failed to load build history:', err);
  }
}

function updateVersionSelectDropdowns() {
  const vSelect = document.getElementById('version-select');
  const targetSelect = document.getElementById('compare-target-select');
  const baseSelect = document.getElementById('compare-base-select');

  if (vSelect) {
    const currentVal = vSelect.value;
    vSelect.innerHTML = '';

    const latestOpt = document.createElement('option');
    latestOpt.value = 'latest';
    latestOpt.innerText = 'Latest Build';
    vSelect.appendChild(latestOpt);

    buildHistory.forEach(cp => {
      const opt = document.createElement('option');
      opt.value = cp.id;
      const baseTag = cp.isBaseline || cp.id === baselineId ? ' 📌 Baseline' : '';
      opt.innerText = `${cp.label || cp.id} (${formatBytes(cp.initialBytes)})${baseTag}`;
      vSelect.appendChild(opt);
    });

    if (currentVal && Array.from(vSelect.options).some(o => o.value === currentVal)) {
      vSelect.value = currentVal;
    }
  }

  // Populate compare selectors
  if (targetSelect && baseSelect) {
    const prevTarget = targetSelect.value;
    const prevBase = baseSelect.value;

    targetSelect.innerHTML = '';
    baseSelect.innerHTML = '';

    buildHistory.forEach(cp => {
      const optT = document.createElement('option');
      optT.value = cp.id;
      optT.innerText = `${cp.label || cp.id} (${formatBytes(cp.initialBytes)})`;
      targetSelect.appendChild(optT);

      const optB = document.createElement('option');
      optB.value = cp.id;
      const baseTag = cp.isBaseline || cp.id === baselineId ? ' 📌 Baseline' : '';
      optB.innerText = `${cp.label || cp.id} (${formatBytes(cp.initialBytes)})${baseTag}`;
      baseSelect.appendChild(optB);
    });

    // Default target: latest checkpoint
    if (buildHistory.length > 0) {
      targetSelect.value = prevTarget || buildHistory[buildHistory.length - 1].id;
    }
    // Default base: baselineId
    if (baselineId) {
      baseSelect.value = prevBase || baselineId;
    }
  }

  // Populate side-by-side selectors
  const sbsTargetSelect = document.getElementById('sbs-target-select');
  const sbsBaseSelect = document.getElementById('sbs-base-select');
  if (sbsTargetSelect && sbsBaseSelect) {
    const prevSbsTarget = sbsTargetSelect.value;
    const prevSbsBase = sbsBaseSelect.value;

    sbsTargetSelect.innerHTML = '';
    sbsBaseSelect.innerHTML = '';

    buildHistory.forEach(cp => {
      const optT = document.createElement('option');
      optT.value = cp.id;
      optT.innerText = `${cp.label || cp.id} (${formatBytes(cp.initialBytes)})`;
      sbsTargetSelect.appendChild(optT);

      const optB = document.createElement('option');
      optB.value = cp.id;
      const baseTag = cp.isBaseline || cp.id === baselineId ? ' 📌 Baseline' : '';
      optB.innerText = `${cp.label || cp.id} (${formatBytes(cp.initialBytes)})${baseTag}`;
      sbsBaseSelect.appendChild(optB);
    });

    if (buildHistory.length > 0) {
      sbsTargetSelect.value = prevSbsTarget || buildHistory[buildHistory.length - 1].id;
    }
    if (baselineId) {
      sbsBaseSelect.value = prevSbsBase || baselineId;
    }
  }
}

async function updateCompareTabBadge() {
  const badge = document.getElementById('drift-summary-badge');
  if (!badge) return;

  if (buildHistory.length < 2) {
    badge.classList.add('hidden');
    return;
  }

  try {
    const base = baselineId || buildHistory[0].id;
    const target = buildHistory[buildHistory.length - 1].id;
    if (base === target) {
      badge.classList.add('hidden');
      return;
    }

    const res = await fetch(`/api/diff?base=${encodeURIComponent(base)}&target=${encodeURIComponent(target)}`);
    if (!res.ok) return;
    const diff = await res.json();

    const d = diff.initialDeltaBytes || 0;
    badge.classList.remove('hidden');
    if (d < 0) {
      badge.innerText = `-${formatBytes(Math.abs(d))}`;
      badge.className = 'px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-[#3fb950]/15 text-[#3fb950] border border-[#3fb950]/30';
    } else if (d > 0) {
      badge.innerText = `+${formatBytes(d)}`;
      badge.className = 'px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30';
    } else {
      badge.innerText = '0 B';
      badge.className = 'px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-[#8b949e]/15 text-[#8b949e] border border-[#30363d]';
    }
  } catch (err) {
    console.error('Failed to update compare badge:', err);
  }
}

// ==========================================
// COMPARE & DRIFT ENGINE
// ==========================================
async function loadAndRenderDiff(baseOverride, targetOverride) {
  const targetSelect = document.getElementById('compare-target-select');
  const baseSelect = document.getElementById('compare-base-select');

  let targetId = targetOverride || (targetSelect ? targetSelect.value : '');
  let baseId = baseOverride || (baseSelect ? baseSelect.value : '');

  if (!baseId && baselineId) baseId = baselineId;
  if (!targetId && buildHistory.length > 0) targetId = buildHistory[buildHistory.length - 1].id;

  if (!baseId || !targetId) return;

  try {
    const res = await fetch(`/api/diff?base=${encodeURIComponent(baseId)}&target=${encodeURIComponent(targetId)}`);
    if (!res.ok) throw new Error(await res.text());
    const diff = await res.json();
    currentDiffData = diff;

    // Top Card 1: Initial JS Drift
    const initDelta = diff.initialDeltaBytes || 0;
    const initPct = diff.initialDeltaPercent || 0;
    const initLargeEl = document.getElementById('drift-init-large');
    const initPctEl = document.getElementById('drift-init-pct');
    const statusBadge = document.getElementById('drift-status-badge');

    if (initLargeEl) {
      if (initDelta < 0) {
        initLargeEl.innerText = `-${formatBytes(Math.abs(initDelta))}`;
        initLargeEl.className = 'text-2xl font-bold font-mono text-[#3fb950]';
        if (initPctEl) initPctEl.innerText = `(${initPct}%) SAVINGS 🎉`;
        if (statusBadge) {
          statusBadge.innerText = 'OPTIMIZATION SAVINGS';
          statusBadge.className = 'px-2 py-0.5 rounded text-[10px] font-semibold bg-[#3fb950]/15 text-[#3fb950] border border-[#3fb950]/30';
        }
      } else if (initDelta > 0) {
        initLargeEl.innerText = `+${formatBytes(initDelta)}`;
        initLargeEl.className = 'text-2xl font-bold font-mono text-[#f85149]';
        if (initPctEl) initPctEl.innerText = `(+${initPct}%) REGRESSION`;
        if (statusBadge) {
          statusBadge.innerText = 'BUNDLE REGRESSION';
          statusBadge.className = 'px-2 py-0.5 rounded text-[10px] font-semibold bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30';
        }
      } else {
        initLargeEl.innerText = '0 B';
        initLargeEl.className = 'text-2xl font-bold font-mono text-[#8b949e]';
        if (initPctEl) initPctEl.innerText = '(0%) UNCHANGED';
        if (statusBadge) {
          statusBadge.innerText = 'IDENTICAL';
          statusBadge.className = 'px-2 py-0.5 rounded text-[10px] font-semibold bg-[#21262d] text-[#8b949e] border border-[#30363d]';
        }
      }
    }

    const baseInitEl = document.getElementById('drift-base-init');
    if (baseInitEl) baseInitEl.innerText = formatBytes(diff.baseInitialBytes || 0);
    const targetInitEl = document.getElementById('drift-target-init');
    if (targetInitEl) targetInitEl.innerText = formatBytes(diff.targetInitialBytes || 0);

    // Top Card 2: Total Build Drift
    const totalDelta = diff.totalDeltaBytes || 0;
    const totalLargeEl = document.getElementById('drift-total-large');
    if (totalLargeEl) {
      if (totalDelta < 0) {
        totalLargeEl.innerText = `-${formatBytes(Math.abs(totalDelta))}`;
        totalLargeEl.className = 'text-2xl font-bold font-mono text-[#3fb950]';
      } else if (totalDelta > 0) {
        totalLargeEl.innerText = `+${formatBytes(totalDelta)}`;
        totalLargeEl.className = 'text-2xl font-bold font-mono text-[#f85149]';
      } else {
        totalLargeEl.innerText = '0 B';
        totalLargeEl.className = 'text-2xl font-bold font-mono text-[#8b949e]';
      }
    }

    const baseCP = buildHistory.find(cp => cp.id === baseId);
    const targetCP = buildHistory.find(cp => cp.id === targetId);
    const baseTotalEl = document.getElementById('drift-base-total');
    if (baseTotalEl) baseTotalEl.innerText = formatBytes(baseCP ? baseCP.totalBytes : 0);
    const targetTotalEl = document.getElementById('drift-target-total');
    if (targetTotalEl) targetTotalEl.innerText = formatBytes(targetCP ? targetCP.totalBytes : 0);

    // Movements breakdown counts
    const pkgs = diff.packageDiffs || [];
    const eliminated = pkgs.filter(p => p.status === 'ELIMINATED').length;
    const reduced = pkgs.filter(p => p.status === 'REDUCED').length;
    const regressed = pkgs.filter(p => p.status === 'REGRESSED').length;
    const added = pkgs.filter(p => p.status === 'ADDED').length;

    const countElim = document.getElementById('drift-count-eliminated');
    if (countElim) countElim.innerText = `${eliminated}`;
    const countRed = document.getElementById('drift-count-reduced');
    if (countRed) countRed.innerText = `${reduced}`;
    const countReg = document.getElementById('drift-count-regressed');
    if (countReg) countReg.innerText = `${regressed}`;
    const countAdd = document.getElementById('drift-count-added');
    if (countAdd) countAdd.innerText = `${added}`;

    const impactEl = document.getElementById('drift-impact-count');
    if (impactEl) impactEl.innerText = `${eliminated + reduced + regressed + added} changed`;

    renderDriftTable(diff);
  } catch (err) {
    console.error('Failed to load diff:', err);
  }
}

function renderDriftTable(diff) {
  const tbody = document.getElementById('drift-table-body');
  if (!tbody || !diff) return;

  let pkgs = diff.packageDiffs || [];

  if (driftFilter === 'changed') {
    pkgs = pkgs.filter(p => p.status !== 'UNCHANGED');
  } else if (driftFilter === 'improved') {
    pkgs = pkgs.filter(p => p.status === 'ELIMINATED' || p.status === 'REDUCED');
  } else if (driftFilter === 'regressed') {
    pkgs = pkgs.filter(p => p.status === 'REGRESSED');
  }

  if (pkgs.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="8" class="py-6 text-center text-xs text-[#8b949e]">
          No package movements match the current filter.
        </td>
      </tr>
    `;
    return;
  }

  tbody.innerHTML = '';
  pkgs.forEach((p, idx) => {
    const tr = document.createElement('tr');
    tr.className = 'hover:bg-[#161b22] transition-colors group';

    const dInit = p.deltaInitialBytes || 0;
    let deltaFormatted = '0 B';
    let deltaClass = 'text-[#8b949e]';

    if (dInit < 0) {
      deltaFormatted = `-${formatBytes(Math.abs(dInit))}`;
      deltaClass = 'text-[#3fb950] font-bold';
    } else if (dInit > 0) {
      deltaFormatted = `+${formatBytes(dInit)}`;
      deltaClass = 'text-[#f85149] font-bold';
    }

    let statusBadge = '<span class="px-2 py-0.5 rounded text-[10px] font-mono bg-[#21262d] text-[#8b949e] border border-[#30363d]">UNCHANGED</span>';
    if (p.status === 'ELIMINATED') {
      statusBadge = '<span class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-[#3fb950]/15 text-[#3fb950] border border-[#3fb950]/30">ELIMINATED</span>';
    } else if (p.status === 'REDUCED') {
      statusBadge = '<span class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-[#58a6ff]/15 text-[#58a6ff] border border-[#58a6ff]/30">REDUCED</span>';
    } else if (p.status === 'REGRESSED') {
      statusBadge = '<span class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30">REGRESSED</span>';
    } else if (p.status === 'ADDED') {
      statusBadge = '<span class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-[#d29922]/15 text-[#d29922] border border-[#d29922]/30">NEW ADDITION</span>';
    }

    // Ingress trail
    let ingressHtml = '<span class="text-[#8b949e]">Root / direct</span>';
    if (p.ingressPath) {
      const parts = p.ingressPath.includes(' → ') ? p.ingressPath.split(' → ') : p.ingressPath.split(' -> ');
      if (parts.length > 1) {
        ingressHtml = `
          <div class="flex items-center space-x-1.5 flex-wrap text-[11px] leading-relaxed py-1">
            <span class="text-[#58a6ff] bg-[#0d1117] px-1.5 py-0.5 rounded border border-[#30363d]">${escapeHTML(parts[0])}</span>
            <span class="text-[#6e7681]">→</span>
            <span class="text-[#e6edf3] font-semibold bg-[#21262d] px-1.5 py-0.5 rounded border border-[#30363d]">${escapeHTML(parts[parts.length - 1])}</span>
          </div>
        `;
      } else {
        ingressHtml = `<span class="text-[#e6edf3] font-semibold">${escapeHTML(parts[0])}</span>`;
      }
    }

    tr.innerHTML = `
      <td class="py-2.5 text-center text-[#8b949e] font-mono text-[11px]">#${idx + 1}</td>
      <td class="py-2.5 font-semibold text-[#e6edf3]">
        <button class="hover:text-[#58a6ff] text-left truncate max-w-[200px] block" data-pkg="${escapeHTML(p.name)}">
          ${escapeHTML(p.name)}
        </button>
      </td>
      <td class="py-2.5 text-right font-mono text-[#8b949e]">${formatBytes(p.baseInitialBytes || 0)}</td>
      <td class="py-2.5 text-right font-mono text-[#e6edf3]">${formatBytes(p.targetInitialBytes || 0)}</td>
      <td class="py-2.5 text-right font-mono ${deltaClass}">${deltaFormatted}</td>
      <td class="py-2.5 text-center">${statusBadge}</td>
      <td class="py-2.5 pl-6 font-mono">${ingressHtml}</td>
      <td class="py-2.5 text-right">
        <button class="inspect-btn px-2 py-1 rounded bg-[#21262d] text-[10px] text-[#c9d1d9] hover:text-[#e6edf3] hover:bg-[#30363d] border border-[#30363d] transition-all" data-pkg="${escapeHTML(p.name)}">
          Inspect ➔
        </button>
      </td>
    `;

    tr.querySelectorAll('button[data-pkg]').forEach(btn => {
      btn.addEventListener('click', () => {
        switchTab('packages');
        selectPackage(p.name);
      });
    });

    tbody.appendChild(tr);
  });
}

// ==========================================
// SIDE-BY-SIDE DIFF ENGINE
// ==========================================
let sbsFilter = 'all';
let sbsSearchQuery = '';
let currentSbsData = null;

async function loadAndRenderSideBySide(baseOverride, targetOverride) {
  const targetSelect = document.getElementById('sbs-target-select');
  const baseSelect = document.getElementById('sbs-base-select');

  let targetId = targetOverride || (targetSelect ? targetSelect.value : '');
  let baseId = baseOverride || (baseSelect ? baseSelect.value : '');

  if (!baseId && baselineId) baseId = baselineId;
  if (!targetId && buildHistory.length > 0) targetId = buildHistory[buildHistory.length - 1].id;

  if (!baseId || !targetId) return;

  try {
    const [baseRes, targetRes, diffRes] = await Promise.all([
      fetch(`/api/bundle?build=${encodeURIComponent(baseId)}`),
      fetch(`/api/bundle?build=${encodeURIComponent(targetId)}`),
      fetch(`/api/diff?base=${encodeURIComponent(baseId)}&target=${encodeURIComponent(targetId)}`)
    ]);

    if (!baseRes.ok || !targetRes.ok || !diffRes.ok) return;

    const baseBundle = await baseRes.json();
    const targetBundle = await targetRes.json();
    const diff = await diffRes.json();

    currentSbsData = { baseBundle, targetBundle, diff, baseId, targetId };

    // Update Header Pill
    const deltaBytes = diff.initialDeltaBytes || 0;
    const deltaPct = diff.initialDeltaPercent || 0;
    const netPill = document.getElementById('sbs-net-delta-pill');
    const netText = document.getElementById('sbs-net-delta-text');

    if (netText) {
      if (deltaBytes < 0) {
        netText.innerText = `-${formatBytes(Math.abs(deltaBytes))} (${deltaPct}%)`;
        if (netPill) netPill.className = 'flex items-center space-x-1.5 px-2.5 py-1 rounded text-xs font-mono font-semibold bg-[#3fb950]/15 text-[#3fb950] border border-[#3fb950]/30';
      } else if (deltaBytes > 0) {
        netText.innerText = `+${formatBytes(deltaBytes)} (+${deltaPct}%)`;
        if (netPill) netPill.className = 'flex items-center space-x-1.5 px-2.5 py-1 rounded text-xs font-mono font-semibold bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30';
      } else {
        netText.innerText = '0 B (0%)';
        if (netPill) netPill.className = 'flex items-center space-x-1.5 px-2.5 py-1 rounded text-xs font-mono font-semibold bg-[#21262d] text-[#8b949e] border border-[#30363d]';
      }
    }

    // Update Column Headers
    const baseCP = buildHistory.find(cp => cp.id === baseId);
    const targetCP = buildHistory.find(cp => cp.id === targetId);

    const baseTitle = document.getElementById('sbs-base-title');
    if (baseTitle) baseTitle.innerText = baseCP ? `${baseCP.label || baseCP.id} (Baseline)` : 'Baseline Build';

    const baseInit = document.getElementById('sbs-base-init-stat');
    if (baseInit) baseInit.innerText = formatBytes(diff.baseInitialBytes || 0);

    const baseTotal = document.getElementById('sbs-base-total-stat');
    if (baseTotal) baseTotal.innerText = formatBytes(baseCP ? baseCP.totalBytes : baseBundle.totalBytes || 0);

    const targetTitle = document.getElementById('sbs-target-title');
    if (targetTitle) targetTitle.innerText = targetCP ? `${targetCP.label || targetCP.id} (Target)` : 'Target Build';

    const targetInit = document.getElementById('sbs-target-init-stat');
    if (targetInit) targetInit.innerText = formatBytes(diff.targetInitialBytes || 0);

    const targetTotal = document.getElementById('sbs-target-total-stat');
    if (targetTotal) targetTotal.innerText = formatBytes(targetCP ? targetCP.totalBytes : targetBundle.totalBytes || 0);

    const targetBadge = document.getElementById('sbs-target-badge');
    if (targetBadge) {
      if (deltaBytes < 0) {
        targetBadge.innerText = `-${formatBytes(Math.abs(deltaBytes))} SAVINGS`;
        targetBadge.className = 'px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-[#3fb950]/15 text-[#3fb950] border border-[#3fb950]/30';
      } else if (deltaBytes > 0) {
        targetBadge.innerText = `+${formatBytes(deltaBytes)} REGRESSION`;
        targetBadge.className = 'px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30';
      } else {
        targetBadge.innerText = 'UNCHANGED';
        targetBadge.className = 'px-2 py-0.5 rounded text-[10px] font-mono font-medium bg-[#21262d] text-[#8b949e] border border-[#30363d]';
      }
    }

    renderSideBySideRows();
  } catch (err) {
    console.error('Failed to load side-by-side diff:', err);
  }
}

function renderSideBySideRows() {
  if (!currentSbsData) return;
  const { baseBundle, targetBundle, diff } = currentSbsData;

  const baseContainer = document.getElementById('sbs-base-rows');
  const targetContainer = document.getElementById('sbs-target-rows');
  if (!baseContainer || !targetContainer) return;

  baseContainer.innerHTML = '';
  targetContainer.innerHTML = '';

  let pkgs = diff.packageDiffs || [];

  if (sbsFilter === 'changed') {
    pkgs = pkgs.filter(p => p.status !== 'UNCHANGED');
  } else if (sbsFilter === 'eliminated') {
    pkgs = pkgs.filter(p => p.status === 'ELIMINATED');
  } else if (sbsFilter === 'regressed') {
    pkgs = pkgs.filter(p => p.status === 'REGRESSED');
  }

  if (sbsSearchQuery) {
    const q = sbsSearchQuery.toLowerCase();
    pkgs = pkgs.filter(p => p.name.toLowerCase().includes(q));
  }

  if (pkgs.length === 0) {
    baseContainer.innerHTML = '<div class="p-8 text-center text-xs font-mono text-[#8b949e]">No packages match filter</div>';
    targetContainer.innerHTML = '<div class="p-8 text-center text-xs font-mono text-[#8b949e]">No packages match filter</div>';
    return;
  }

  // Create lookup maps for fast access to rich bundle details
  const baseMap = new Map();
  (baseBundle.topPackages || []).forEach(p => baseMap.set(p.name, p));
  const targetMap = new Map();
  (targetBundle.topPackages || []).forEach(p => targetMap.set(p.name, p));

  pkgs.forEach((p, idx) => {
    const basePkg = baseMap.get(p.name);
    const targetPkg = targetMap.get(p.name);
    const safePkg = escapeHTML(p.name);

    // Left Column (Baseline) Row
    const leftDiv = document.createElement('div');
    leftDiv.className = 'sbs-row p-3 flex items-center justify-between text-xs cursor-pointer hover:bg-[#161b22] border-l-2 border-transparent';
    leftDiv.dataset.sbsPkg = p.name;

    const baseInitBytes = p.baseInitialBytes || 0;
    const basePresent = baseInitBytes > 0 || (basePkg && basePkg.sizeBytes > 0);

    leftDiv.innerHTML = `
      <div class="flex items-center space-x-2.5 min-w-0 pr-3">
        <span class="text-[10px] font-mono text-[#8b949e] w-5 shrink-0 text-right">#${idx + 1}</span>
        <div class="min-w-0">
          <div class="font-semibold text-[#e6edf3] truncate">${safePkg}</div>
          <div class="text-[10px] font-mono text-[#8b949e] truncate mt-0.5">
            ${basePresent ? (basePkg?.chunks?.[0] || 'initial') : '<span class="text-[#6e7681]">Not in baseline</span>'}
          </div>
        </div>
      </div>
      <div class="text-right shrink-0 font-mono">
        <div class="font-bold ${baseInitBytes > 100 * 1024 ? 'text-[#f85149]' : 'text-[#e6edf3]'}">
          ${baseInitBytes > 0 ? formatBytes(baseInitBytes) : (basePresent ? '<span class="text-[#58a6ff]">async</span>' : '<span class="text-[#6e7681]">--</span>')}
        </div>
        <div class="text-[10px] text-[#8b949e]">
          ${basePkg ? `${formatBytes(basePkg.sizeBytes)} total` : ''}
        </div>
      </div>
    `;

    // Right Column (Target) Row
    const rightDiv = document.createElement('div');
    rightDiv.className = 'sbs-row p-3 flex items-center justify-between text-xs cursor-pointer hover:bg-[#161b22] border-l-2 border-transparent';
    rightDiv.dataset.sbsPkg = p.name;

    const targetInitBytes = p.targetInitialBytes || 0;
    let statusPill = '';
    let deltaDisplay = '';

    if (p.status === 'ELIMINATED') {
      statusPill = `<span class="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-[#3fb950]/15 text-[#3fb950] border border-[#3fb950]/30">ELIMINATED</span>`;
      deltaDisplay = `<span class="text-[#3fb950] font-bold">-${formatBytes(p.baseInitialBytes)}</span>`;
    } else if (p.status === 'REDUCED') {
      statusPill = `<span class="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-[#58a6ff]/15 text-[#58a6ff] border border-[#58a6ff]/30">REDUCED</span>`;
      deltaDisplay = `<span class="text-[#3fb950] font-bold">${formatBytes(targetInitBytes)} (-${formatBytes(Math.abs(p.deltaBytes))})</span>`;
    } else if (p.status === 'REGRESSED') {
      statusPill = `<span class="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30">REGRESSED</span>`;
      deltaDisplay = `<span class="text-[#f85149] font-bold">${formatBytes(targetInitBytes)} (+${formatBytes(p.deltaBytes)})</span>`;
    } else if (p.status === 'ADDED') {
      statusPill = `<span class="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-[#d29922]/15 text-[#d29922] border border-[#d29922]/30">NEW</span>`;
      deltaDisplay = `<span class="text-[#d29922] font-bold">${formatBytes(targetInitBytes)} (+${formatBytes(p.deltaBytes)})</span>`;
    } else {
      statusPill = `<span class="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-[#21262d] text-[#8b949e] border border-[#30363d]">UNCHANGED</span>`;
      deltaDisplay = `<span class="text-[#8b949e] font-medium">${formatBytes(targetInitBytes)}</span>`;
    }

    rightDiv.innerHTML = `
      <div class="flex items-center space-x-2.5 min-w-0 pr-3">
        <span class="text-[10px] font-mono text-[#8b949e] w-5 shrink-0 text-right">#${idx + 1}</span>
        <div class="min-w-0">
          <div class="font-semibold text-[#e6edf3] truncate flex items-center space-x-2">
            <span>${safePkg}</span>
            ${statusPill}
          </div>
          <div class="text-[10px] font-mono text-[#8b949e] truncate mt-0.5">
            ${p.status === 'ELIMINATED' ? '<span class="text-[#3fb950]">Removed from initial bundle</span>' : (targetPkg?.chunks?.[0] || 'initial')}
          </div>
        </div>
      </div>
      <div class="text-right shrink-0 font-mono">
        <div class="text-xs">${deltaDisplay}</div>
        <div class="text-[10px] text-[#8b949e]">
          ${targetPkg ? `${formatBytes(targetPkg.sizeBytes)} total` : (p.status === 'ELIMINATED' ? '0 B total' : '')}
        </div>
      </div>
    `;

    // Synchronized Hover Highlight
    const onEnter = () => {
      document.querySelectorAll(`.sbs-row[data-sbs-pkg="${p.name}"]`).forEach(el => el.classList.add('sbs-highlight'));
    };
    const onLeave = () => {
      document.querySelectorAll(`.sbs-row[data-sbs-pkg="${p.name}"]`).forEach(el => el.classList.remove('sbs-highlight'));
    };

    leftDiv.addEventListener('mouseenter', onEnter);
    leftDiv.addEventListener('mouseleave', onLeave);
    rightDiv.addEventListener('mouseenter', onEnter);
    rightDiv.addEventListener('mouseleave', onLeave);

    // Click to view Ingress Comparison
    const onClick = () => showSbsDetail(p.name, basePkg, targetPkg, p);
    leftDiv.addEventListener('click', onClick);
    rightDiv.addEventListener('click', onClick);

    baseContainer.appendChild(leftDiv);
    targetContainer.appendChild(rightDiv);
  });
}

function showSbsDetail(pkgName, basePkg, targetPkg, diffEntry) {
  const panel = document.getElementById('sbs-detail-panel');
  const nameEl = document.getElementById('sbs-detail-pkg-name');
  const baseIngress = document.getElementById('sbs-detail-base-ingress');
  const targetIngress = document.getElementById('sbs-detail-target-ingress');

  if (!panel || !nameEl || !baseIngress || !targetIngress) return;

  nameEl.innerText = pkgName;

  // Render Baseline Ingress
  const baseTrail = basePkg?.ingressPath || diffEntry?.ingressPath;
  if (baseTrail) {
    const parts = baseTrail.includes(' → ') ? baseTrail.split(' → ') : baseTrail.split(' -> ');
    baseIngress.innerHTML = parts.map((part, i) => {
      const isLast = i === parts.length - 1;
      return `<div class="py-0.5 ${isLast ? 'text-[#e6edf3] font-bold' : 'text-[#8b949e]'}">${isLast ? '●' : '↓'} ${escapeHTML(part.trim())}</div>`;
    }).join('');
  } else {
    baseIngress.innerHTML = '<span class="text-[#8b949e]">No ingress trail recorded (or not in baseline)</span>';
  }

  // Render Target Ingress
  if (diffEntry?.status === 'ELIMINATED') {
    targetIngress.innerHTML = '<div class="text-[#3fb950] font-semibold flex items-center space-x-1.5"><span>✓</span><span>Ingress chain eliminated! Package is no longer bundled in initial load.</span></div>';
  } else if (targetPkg?.ingressPath) {
    const parts = targetPkg.ingressPath.includes(' → ') ? targetPkg.ingressPath.split(' → ') : targetPkg.ingressPath.split(' -> ');
    targetIngress.innerHTML = parts.map((part, i) => {
      const isLast = i === parts.length - 1;
      return `<div class="py-0.5 ${isLast ? 'text-[#58a6ff] font-bold' : 'text-[#8b949e]'}">${isLast ? '●' : '↓'} ${escapeHTML(part.trim())}</div>`;
    }).join('');
  } else {
    targetIngress.innerHTML = '<span class="text-[#8b949e]">Direct entrypoint or lazy-loaded module</span>';
  }

  panel.classList.remove('hidden');
  panel.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

// ==========================================
// LOAD & RENDER BUNDLE
// ==========================================
async function loadBundle(buildId = 'latest') {
  try {
    const url = buildId && buildId !== 'latest' ? `/api/bundle?build=${encodeURIComponent(buildId)}` : '/api/bundle';
    const res = await fetch(url);
    if (!res.ok) throw new Error(await res.text());
    bundleData = await res.json();
    renderBundle(bundleData);
  } catch (err) {
    console.error('Failed to load bundle data:', err);
    const badge = document.getElementById('stats-badge');
    if (badge) badge.innerText = 'No stats loaded';
  }
}

function renderBundle(data) {
  const badge = document.getElementById('stats-badge');
  if (badge) badge.innerText = data.statsPath || 'stats.json';

  const mainEp = data.entrypoints && (data.entrypoints['main'] || Object.values(data.entrypoints)[0]);
  const initialBytes = mainEp ? mainEp.initialBytes : 0;
  const asyncBytes = mainEp ? mainEp.asyncBytes : 0;
  const totalBytes = data.totalBytes || 0;
  const packages = data.topPackages || [];
  const chunks = data.chunks || [];

  const initialChunks = chunks.filter(c => c.type === 'initial');
  const asyncChunks = chunks.filter(c => c.type === 'async');
  const initialChunksTotal = initialChunks.reduce((acc, c) => acc + c.sizeBytes, 0) || initialBytes;
  const asyncChunksTotal = asyncChunks.reduce((acc, c) => acc + c.sizeBytes, 0) || asyncBytes;
  const totalGzip = chunks.reduce((acc, c) => acc + (c.gzipBytes || 0), 0);

  const multiChunkPackages = packages.filter(p => p.chunks && p.chunks.length > 1);

  // Update top metrics in diagnosis view
  const initLargeEl = document.getElementById('metric-initial-large');
  if (initLargeEl) initLargeEl.innerText = formatBytes(initialBytes);

  const chunksCountEl = document.getElementById('metric-chunks-count');
  if (chunksCountEl) chunksCountEl.innerText = `${chunks.length} chunks`;

  const asyncLargeEl = document.getElementById('metric-async-large');
  if (asyncLargeEl) asyncLargeEl.innerText = formatBytes(asyncChunksTotal);

  const initChunksEl = document.getElementById('metric-initial-chunks');
  if (initChunksEl) initChunksEl.innerText = `${initialChunks.length} (${formatBytes(initialChunksTotal)})`;

  const lazyChunksEl = document.getElementById('metric-lazy-chunks');
  if (lazyChunksEl) lazyChunksEl.innerText = `${asyncChunks.length} (${formatBytes(asyncChunksTotal)})`;

  const pkgsCountEl = document.getElementById('metric-pkgs-count');
  if (pkgsCountEl) pkgsCountEl.innerText = `${packages.length} pkgs`;

  const totalLargeEl = document.getElementById('metric-total-large');
  if (totalLargeEl) totalLargeEl.innerText = formatBytes(totalBytes);

  const totalGzipEl = document.getElementById('metric-total-gzip');
  if (totalGzipEl) totalGzipEl.innerText = totalGzip > 0 ? `~${formatBytes(totalGzip)} gzip` : '-- gzip';

  const multiChunkCountEl = document.getElementById('metric-multichunk-count');
  if (multiChunkCountEl) multiChunkCountEl.innerText = `${multiChunkPackages.length}`;

  const initPctEl = document.getElementById('metric-initial-pct');
  if (initPctEl) {
    const pct = totalBytes > 0 ? Math.round((initialBytes / totalBytes) * 100) : 0;
    initPctEl.innerText = `${pct}%`;
  }

  // Health Status Badge & Budget comparison
  const budget = 500 * 1024;
  const statusBadge = document.getElementById('health-status-badge');
  const healthMeter = document.getElementById('health-meter');
  const healthDesc = document.getElementById('health-description');

  if (initialBytes > budget * 2) {
    if (statusBadge) {
      statusBadge.innerText = 'CRITICAL BLOAT';
      statusBadge.className = 'px-2 py-0.5 rounded text-[10px] font-semibold bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30';
    }
    if (healthMeter) {
      healthMeter.className = 'bg-[#f85149] h-full rounded-full transition-all duration-500';
      healthMeter.style.width = '100%';
    }
    const overPct = (((initialBytes - budget) / budget) * 100).toFixed(0);
    if (healthDesc) healthDesc.innerText = `Exceeds 500 KB budget by +${formatBytes(initialBytes - budget)} (+${overPct}%). Initial payload blocks fast first paint.`;
  } else if (initialBytes > budget) {
    if (statusBadge) {
      statusBadge.innerText = 'OVER BUDGET';
      statusBadge.className = 'px-2 py-0.5 rounded text-[10px] font-semibold bg-[#d29922]/15 text-[#d29922] border border-[#d29922]/30';
    }
    if (healthMeter) {
      healthMeter.className = 'bg-[#d29922] h-full rounded-full transition-all duration-500';
      healthMeter.style.width = '75%';
    }
    const overPct = (((initialBytes - budget) / budget) * 100).toFixed(0);
    if (healthDesc) healthDesc.innerText = `Exceeds 500 KB budget by +${formatBytes(initialBytes - budget)} (+${overPct}%).`;
  } else {
    if (statusBadge) {
      statusBadge.innerText = 'OPTIMAL HEALTH';
      statusBadge.className = 'px-2 py-0.5 rounded text-[10px] font-semibold bg-[#3fb950]/15 text-[#3fb950] border border-[#3fb950]/30';
    }
    if (healthMeter) {
      healthMeter.className = 'bg-[#3fb950] h-full rounded-full transition-all duration-500';
      healthMeter.style.width = `${Math.round((initialBytes / budget) * 100)}%`;
    }
    if (healthDesc) healthDesc.innerText = `Initial bootstrap payload is within healthy budgets (${formatBytes(initialBytes)} / 500 KB).`;
  }

  // Populate Chunk Select options
  const select = document.getElementById('chunk-select');
  if (select) {
    select.innerHTML = '';
    const allOpt = document.createElement('option');
    allOpt.value = 'all';
    allOpt.innerText = `All Chunks (${formatBytes(totalBytes)})`;
    select.appendChild(allOpt);

    for (const ch of chunks) {
      const opt = document.createElement('option');
      opt.value = ch.name;
      opt.innerText = `${ch.name} (${formatBytes(ch.sizeBytes)}) [${ch.type}]`;
      select.appendChild(opt);
    }

    select.addEventListener('change', () => {
      const val = select.value;
      const pill = document.getElementById('active-chunk-pill');
      if (pill) {
        pill.innerText = select.options[select.selectedIndex]?.text.split(' (')[0] || val;
      }
      updateActiveChunks(val);
    });
  }

  // Filter initial packages: strictly packages with physical bytes in initial chunks
  const initialContributors = packages.filter(p => (p.initialBytes || 0) > 0);
  initialContributors.sort((a, b) => (b.initialBytes || 0) - (a.initialBytes || 0));

  const diagBadge = document.getElementById('diag-count-badge');
  if (diagBadge) diagBadge.innerText = `${initialContributors.length} in initial`;

  renderInitialContributors(initialContributors, initialBytes);
  renderChunksInventory(chunks, totalBytes);

  currentPackages = packages;
  treemap.setData(packages, totalBytes);
  applyFilterAndSearch();

  if (packages.length > 0) {
    selectPackage(packages[0].name);
  }
}

function renderInitialContributors(contributors, initialBytes) {
  const tbody = document.getElementById('initial-contributors-body');
  if (!tbody) return;

  if (contributors.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="7" class="py-6 text-center text-xs text-[#3fb950]">
          No third-party packages in initial chunks.
        </td>
      </tr>
    `;
    return;
  }

  tbody.innerHTML = '';
  contributors.forEach((pkg, idx) => {
    const tr = document.createElement('tr');
    tr.className = 'hover:bg-[#161b22] transition-colors group';

    const initSize = pkg.initialBytes || 0;
    const pctOfInitial = initialBytes > 0 ? ((initSize / initialBytes) * 100).toFixed(1) : '0';
    const isHeavy = initSize >= 100 * 1024;
    const sizeColor = isHeavy ? 'text-[#f85149]' : 'text-[#d29922]';
    const estimatedGzip = Math.round(initSize * 0.30);

    // Format ingress trail
    let ingressHtml = '<span class="text-[#8b949e]">Direct / root entrypoint import</span>';
    if (pkg.ingressPath) {
      const parts = pkg.ingressPath.includes(' → ') ? pkg.ingressPath.split(' → ') : pkg.ingressPath.split(' -> ');
      if (parts.length === 1) {
        ingressHtml = `<span class="text-[#e6edf3] font-semibold">${escapeHTML(parts[0])}</span>`;
      } else {
        const rootFile = parts[0];
        const intermediate = parts.slice(1, parts.length - 1);
        const target = parts[parts.length - 1];

        ingressHtml = `
          <div class="flex items-center space-x-1.5 flex-wrap text-[11px] leading-relaxed py-1">
            <span class="text-[#58a6ff] bg-[#0d1117] px-1.5 py-0.5 rounded border border-[#30363d]">${escapeHTML(rootFile)}</span>
            ${intermediate.length > 0 ? `<span class="text-[#6e7681]">→</span><span class="text-[#8b949e] truncate max-w-[200px]" title="${escapeHTML(intermediate.join(' → '))}">${escapeHTML(intermediate[intermediate.length - 1])}</span>` : ''}
            <span class="text-[#6e7681]">→</span>
            <span class="text-[#e6edf3] font-semibold bg-[#21262d] px-1.5 py-0.5 rounded border border-[#30363d]">${escapeHTML(target)}</span>
          </div>
        `;
      }
    }

    const asyncTag = (pkg.asyncBytes || 0) > 0 
      ? `<div class="text-[10px] text-[#58a6ff] font-normal mt-0.5">+${formatBytes(pkg.asyncBytes)} in async</div>` 
      : '';

    tr.innerHTML = `
      <td class="py-2.5 text-center text-[#8b949e] font-mono text-[11px]">#${idx + 1}</td>
      <td class="py-2.5 font-semibold text-[#e6edf3]">
        <button class="hover:text-[#58a6ff] text-left truncate max-w-[200px] block" data-pkg="${escapeHTML(pkg.name)}">
          ${escapeHTML(pkg.name)}
        </button>
      </td>
      <td class="py-2.5 text-right font-bold ${sizeColor}">
        <div>${formatBytes(initSize)}</div>
        ${asyncTag}
      </td>
      <td class="py-2.5 text-right text-[#8b949e]">~${formatBytes(estimatedGzip)}</td>
      <td class="py-2.5 text-right font-medium ${isHeavy ? 'text-[#f85149]' : 'text-[#c9d1d9]'}">${pctOfInitial}%</td>
      <td class="py-2.5 pl-6 font-mono">${ingressHtml}</td>
      <td class="py-2.5 text-right">
        <button class="inspect-btn px-2 py-1 rounded bg-[#21262d] text-[10px] text-[#c9d1d9] hover:text-[#e6edf3] hover:bg-[#30363d] border border-[#30363d] transition-all" data-pkg="${escapeHTML(pkg.name)}">
          Inspect ➔
        </button>
      </td>
    `;

    tr.querySelectorAll('button[data-pkg]').forEach(btn => {
      btn.addEventListener('click', () => {
        switchTab('packages');
        selectPackage(pkg.name);
      });
    });

    tbody.appendChild(tr);
  });
}

function renderChunksInventory(chunks, totalBytes) {
  const tbody = document.getElementById('chunks-inventory-body');
  if (!tbody) return;

  tbody.innerHTML = '';
  chunks.forEach(ch => {
    const tr = document.createElement('tr');
    tr.className = 'hover:bg-[#161b22] transition-colors';

    const isInitial = (ch.type === 'initial');
    const typeBadge = isInitial 
      ? '<span class="px-2 py-0.5 rounded text-[10px] font-semibold bg-[#f85149]/15 text-[#f85149] border border-[#f85149]/30">Initial</span>'
      : '<span class="px-2 py-0.5 rounded text-[10px] font-semibold bg-[#58a6ff]/15 text-[#58a6ff] border border-[#58a6ff]/30">Async / Lazy</span>';

    const pct = totalBytes > 0 ? ((ch.sizeBytes / totalBytes) * 100).toFixed(1) : '0';

    tr.innerHTML = `
      <td class="py-2.5 font-semibold text-[#e6edf3] flex items-center space-x-2">
        <span class="w-1.5 h-1.5 rounded-full ${isInitial ? 'bg-[#f85149]' : 'bg-[#58a6ff]'}"></span>
        <span>${escapeHTML(ch.name)}</span>
      </td>
      <td class="py-2.5">${typeBadge}</td>
      <td class="py-2.5 text-right font-bold ${isInitial ? 'text-[#f85149]' : 'text-[#58a6ff]'}">${formatBytes(ch.sizeBytes)}</td>
      <td class="py-2.5 text-right text-[#8b949e]">~${formatBytes(ch.gzipBytes || Math.round(ch.sizeBytes * 0.3))}</td>
      <td class="py-2.5 text-right text-[#8b949e]">${pct}%</td>
    `;
    tbody.appendChild(tr);
  });
}

function updateActiveChunks(chunkName) {
  if (!bundleData) return;
  if (chunkName === 'all') {
    treemap.setData(bundleData.topPackages || [], bundleData.totalBytes || 1);
    currentPackages = bundleData.topPackages || [];
    applyFilterAndSearch();
    return;
  }

  const filtered = (bundleData.topPackages || []).filter(p => p.chunks && p.chunks.includes(chunkName));
  const chunkTotal = (bundleData.chunks || []).find(c => c.name === chunkName)?.sizeBytes || 1;
  treemap.setData(filtered, chunkTotal);
  currentPackages = filtered;
  applyFilterAndSearch();

  if (filtered.length > 0) {
    selectPackage(filtered[0].name);
  }
}

function applyFilterAndSearch() {
  const q = (document.getElementById('search-input')?.value || '').toLowerCase().trim();
  let list = [...currentPackages];

  if (currentFilter === 'heavy') {
    list = list.filter(p => p.sizeBytes >= 100 * 1024);
  } else if (currentFilter === 'initial') {
    list = list.filter(p => (p.initialBytes || 0) > 0);
  } else if (currentFilter === 'async') {
    list = list.filter(p => (p.initialBytes || 0) === 0 && (p.asyncBytes || 0) > 0);
  }

  if (q) {
    list = list.filter(p => p.name.toLowerCase().includes(q));
  }

  renderExplorerList(list);
}

function renderExplorerList(packages) {
  const container = document.getElementById('explorer-package-list');
  const countEl = document.getElementById('explorer-count');
  if (countEl) countEl.innerText = `${packages.length}`;
  if (!container) return;

  if (packages.length === 0) {
    container.innerHTML = '<div class="p-6 text-center text-xs font-mono text-[#8b949e]">No matching packages found</div>';
    return;
  }

  container.innerHTML = '';
  const totalSize = bundleData?.totalBytes || 1;

  packages.forEach((p, idx) => {
    const div = document.createElement('div');
    const isSelected = (p.name === selectedPkgName);
    div.className = `p-3 flex items-center justify-between cursor-pointer transition-colors package-item hover:bg-[#161b22] ${isSelected ? 'row-selected' : ''}`;
    div.dataset.name = p.name.toLowerCase();

    const pct = ((p.sizeBytes / totalSize) * 100).toFixed(1);
    const safeName = escapeHTML(p.name);

    div.innerHTML = `
      <div class="flex items-center space-x-3 min-w-0 pr-2">
        <span class="text-[10px] font-mono text-[#8b949e] w-5 shrink-0 text-right">#${idx + 1}</span>
        <div class="min-w-0">
          <div class="text-xs font-semibold text-[#e6edf3] truncate">${safeName}</div>
          <div class="text-[10px] font-mono text-[#8b949e] truncate mt-0.5">
            ${(p.chunks && p.chunks.length) ? escapeHTML(p.chunks[0]) : 'chunk'}
          </div>
        </div>
      </div>
      <div class="text-right shrink-0 font-mono">
        <div class="text-xs font-bold text-[#d29922]">${formatBytes(p.sizeBytes)}</div>
        <div class="text-[10px] text-[#8b949e]">${pct}%</div>
      </div>
    `;

    div.addEventListener('click', () => {
      selectPackage(p.name);
    });

    container.appendChild(div);
  });
}

function selectPackage(name) {
  if (!name || !bundleData) return;
  selectedPkgName = name;
  treemap.setSelected(name);

  const pkg = (bundleData.topPackages || []).find(p => p.name === name);
  if (!pkg) return;

  const nameEl = document.getElementById('exp-pkg-name');
  if (nameEl) nameEl.innerText = pkg.name;

  const sizeEl = document.getElementById('exp-pkg-size');
  if (sizeEl) sizeEl.innerText = formatBytes(pkg.sizeBytes);

  const initEl = document.getElementById('exp-initial');
  if (initEl) initEl.innerText = formatBytes(pkg.initialBytes || 0);

  const asyncEl = document.getElementById('exp-async');
  if (asyncEl) asyncEl.innerText = formatBytes(pkg.asyncBytes || 0);

  const rawEl = document.getElementById('exp-raw');
  if (rawEl) rawEl.innerText = formatBytes(pkg.sizeBytes);

  const gzipEl = document.getElementById('exp-gzip');
  if (gzipEl) {
    if (pkg.gzipBytes) {
      const saved = (((pkg.sizeBytes - pkg.gzipBytes) / pkg.sizeBytes) * 100).toFixed(0);
      gzipEl.innerHTML = `~${formatBytes(pkg.gzipBytes)} <span class="text-[10px] text-[#3fb950] font-normal">(-${saved}%)</span>`;
    } else {
      gzipEl.innerText = '--';
    }
  }

  const chunksEl = document.getElementById('exp-chunks');
  if (chunksEl) {
    chunksEl.innerHTML = '';
    if (pkg.chunks && pkg.chunks.length > 0) {
      pkg.chunks.forEach(c => {
        const span = document.createElement('span');
        span.className = 'px-1.5 py-0.5 rounded bg-[#0d1117] text-[#c9d1d9] border border-[#30363d]';
        span.innerText = c;
        chunksEl.appendChild(span);
      });
    } else {
      chunksEl.innerHTML = '<span class="text-[#8b949e] text-xs">No chunks attributed</span>';
    }
  }

  const ingressEl = document.getElementById('exp-ingress');
  if (ingressEl) {
    if (pkg.ingressPath) {
      const parts = pkg.ingressPath.includes(' → ') ? pkg.ingressPath.split(' → ') : pkg.ingressPath.split(' -> ');
      ingressEl.innerHTML = '';
      parts.forEach((part, i) => {
        const isLast = (i === parts.length - 1);
        const nodeDiv = document.createElement('div');
        nodeDiv.className = 'ingress-node flex items-start space-x-2 py-1';
        nodeDiv.innerHTML = `
          <span class="text-[10px] ${isLast ? 'text-[#d29922] font-bold' : 'text-[#58a6ff]'} mt-0.5">${isLast ? '●' : '↓'}</span>
          <span class="break-all ${isLast ? 'text-[#e6edf3] font-semibold' : 'text-[#8b949e]'}">${escapeHTML(part.trim())}</span>
        `;
        ingressEl.appendChild(nodeDiv);
      });
    } else {
      if ((pkg.initialBytes || 0) === 0 && (pkg.asyncBytes || 0) > 0) {
        ingressEl.innerHTML = '<span class="text-[#58a6ff] font-semibold">⚡ Lazy-Loaded Dependency (Not in Initial Bundle)</span>';
      } else {
        ingressEl.innerHTML = '<span class="text-[#8b949e]">Root dependency or direct entrypoint import</span>';
      }
    }
  }

  document.querySelectorAll('.package-item').forEach(item => {
    if (item.dataset.name === name.toLowerCase()) {
      item.classList.add('row-selected');
      item.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    } else {
      item.classList.remove('row-selected');
    }
  });
}

function escapeHTML(str) {
  return (str || '').replace(/[&<>"']/g, m => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  })[m]);
}

function formatBytes(b) {
  if (!b) return '0 B';
  if (b >= 1048576) return (b / 1048576).toFixed(2) + ' MB';
  if (b >= 1024) return (b / 1024).toFixed(1) + ' KB';
  return b + ' B';
}

window.addEventListener('DOMContentLoaded', init);
