class TreemapEngine {
  constructor(canvasId, tooltipId) {
    this.canvas = document.getElementById(canvasId);
    this.ctx = this.canvas.getContext('2d');
    this.tooltip = document.getElementById(tooltipId);
    this.nodes = [];
    this.hoveredNode = null;
    this.selectedName = '';
    this.highlightQuery = '';
    this.onSelect = null;
    this.onDrillDown = null;
    this.onDrillUp = null;
    this.useGzip = false;

    this.drillStack = [];
    this.currentLevel = null;

    this.palette = [
      '#2563eb', '#059669', '#7c3aed', '#d97706', '#0284c7',
      '#e11d48', '#0d9488', '#c026d3', '#ea580c', '#4f46e5',
      '#16a34a', '#0369a1', '#9333ea', '#b45309', '#dc2626',
    ];

    this.packageColorMap = new Map();
    this.initEvents();
  }

  initEvents() {
    window.addEventListener('resize', () => this.resizeAndDraw());
    if (window.ResizeObserver && this.canvas.parentElement) {
      new ResizeObserver(() => this.resizeAndDraw()).observe(this.canvas.parentElement);
    }

    this.canvas.addEventListener('mousemove', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      const hit = this.hitTest(x, y);
      if (hit !== this.hoveredNode) {
        this.hoveredNode = hit;
        this.draw();
        this.updateTooltip(hit, e.clientX, e.clientY);
      } else if (hit) {
        this.updateTooltip(hit, e.clientX, e.clientY);
      }
    });

    this.canvas.addEventListener('mouseleave', () => {
      this.hoveredNode = null;
      this.tooltip.classList.add('hidden');
      this.draw();
    });

    this.canvas.addEventListener('click', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      const hit = this.hitTest(x, y);
      if (!hit) return;

      if (hit.item.isOther) return;

      if (hit.item.children && hit.item.children.length > 0) {
        this.drillInto(hit.item);
      } else if (typeof this.onSelect === 'function') {
        this.selectedName = hit.item.name;
        this.draw();
        this.onSelect(hit.item);
      }
    });

    this.canvas.addEventListener('contextmenu', (e) => {
      e.preventDefault();
      this.drillUp();
    });
  }

  resizeAndDraw() {
    if (!this.canvas || !this.canvas.parentElement) return;
    const rect = this.canvas.parentElement.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    this.canvas.width = rect.width * dpr;
    this.canvas.height = rect.height * dpr;
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    this.displayWidth = rect.width;
    this.displayHeight = rect.height;
    this.computeLayout();
    this.draw();
  }

  sizeOf(item) {
    return this.useGzip ? (item.gzipBytes || item.sizeBytes || 0) : (item.sizeBytes || 0);
  }

  setData(packages, totalBytes, modulesByPackage) {
    this.rawPackages = packages || [];
    this.modulesByPackage = modulesByPackage || {};
    this.totalBytes = totalBytes || this.rawPackages.reduce((acc, p) => acc + (p.sizeBytes || 0), 0);
    this.drillStack = [];
    this.currentLevel = null;

    this.packageColorMap.clear();
    const sorted = [...this.rawPackages].sort((a, b) => (b.sizeBytes || 0) - (a.sizeBytes || 0));
    sorted.forEach((p, i) => {
      if (!p.isOther) this.packageColorMap.set(p.name, this.palette[i % this.palette.length]);
    });

    this._buildLevel(this.rawPackages, this.totalBytes, null);
  }

  _buildLevel(items, totalBytes, parentName) {
    const threshold = totalBytes * 0.002;
    const major = [];
    let otherBytes = 0;
    let otherGzip = 0;
    let otherCount = 0;

    const sorted = [...items].sort((a, b) => this.sizeOf(b) - this.sizeOf(a));

    for (const p of sorted) {
      if (this.sizeOf(p) >= threshold) {
        const entry = { ...p };
        if (!parentName && this.modulesByPackage[p.name] && this.modulesByPackage[p.name].length > 1) {
          entry.children = this.modulesByPackage[p.name];
        }
        major.push(entry);
      } else {
        otherBytes += p.sizeBytes || 0;
        otherGzip += p.gzipBytes || 0;
        otherCount++;
      }
    }

    if (otherCount > 0) {
      major.push({
        name: `(${otherCount} smaller ${parentName ? 'modules' : 'packages'})`,
        sizeBytes: otherBytes,
        gzipBytes: otherGzip,
        isOther: true,
      });
    }

    this.currentLevel = { items: major, totalBytes, parentName };
    this.resizeAndDraw();
  }

  drillInto(item) {
    if (!item.children || item.children.length === 0) return;

    this.drillStack.push(this.currentLevel);
    const childTotal = item.children.reduce((sum, c) => sum + (c.sizeBytes || 0), 0);
    this._buildLevel(item.children, childTotal, item.name);

    if (typeof this.onDrillDown === 'function') {
      this.onDrillDown(item.name, this.drillStack.length);
    }
  }

  drillUp() {
    if (this.drillStack.length === 0) return;
    this.currentLevel = this.drillStack.pop();
    this.resizeAndDraw();

    if (typeof this.onDrillUp === 'function') {
      this.onDrillUp(this.currentLevel.parentName, this.drillStack.length);
    }
  }

  drillToRoot() {
    if (this.drillStack.length === 0) return;
    this.currentLevel = this.drillStack[0];
    this.drillStack = [];
    this.resizeAndDraw();

    if (typeof this.onDrillUp === 'function') {
      this.onDrillUp(null, 0);
    }
  }

  get depth() {
    return this.drillStack.length;
  }

  computeLayout() {
    if (!this.currentLevel || !this.currentLevel.items.length || !this.displayWidth || !this.displayHeight) {
      this.nodes = [];
      return;
    }
    const items = this.currentLevel.items;
    this.nodes = squarify(items, 0, 0, this.displayWidth, this.displayHeight, (i) => this.sizeOf(i));
  }

  draw() {
    if (!this.ctx || !this.displayWidth) return;
    this.ctx.clearRect(0, 0, this.displayWidth, this.displayHeight);

    const isModuleLevel = this.drillStack.length > 0;
    const parentPkgName = this.currentLevel?.parentName;

    for (let i = 0; i < this.nodes.length; i++) {
      const node = this.nodes[i];
      if (node.w < 1 || node.h < 1) continue;

      const isHovered = (node === this.hoveredNode);
      const isSelected = (this.selectedName && node.item.name === this.selectedName);
      const isMatch = this.highlightQuery && node.item.name.toLowerCase().includes(this.highlightQuery);
      const hasDrill = !!(node.item.children && node.item.children.length > 0);

      this.ctx.save();

      let baseColor;
      if (node.item.isOther) {
        baseColor = '#475569';
      } else if (isModuleLevel) {
        const pkgColor = this.packageColorMap.get(parentPkgName) || this.palette[0];
        baseColor = shadeColor(pkgColor, -15 + (i % 5) * 8);
      } else {
        baseColor = this.packageColorMap.get(node.item.name) || this.palette[i % this.palette.length];
      }

      this.ctx.fillStyle = baseColor;

      if (this.highlightQuery && !isMatch) {
        this.ctx.globalAlpha = 0.2;
      } else if (isHovered) {
        this.ctx.globalAlpha = 0.85;
      } else {
        this.ctx.globalAlpha = 1.0;
      }

      const pad = 1;
      const rx = node.x + pad;
      const ry = node.y + pad;
      const rw = Math.max(0, node.w - pad * 2);
      const rh = Math.max(0, node.h - pad * 2);

      roundRect(this.ctx, rx, ry, rw, rh, 3);
      this.ctx.fill();

      if (isSelected) {
        this.ctx.strokeStyle = '#ffffff';
        this.ctx.lineWidth = 2.5;
        roundRect(this.ctx, rx, ry, rw, rh, 3);
        this.ctx.stroke();
      } else if (isHovered) {
        this.ctx.strokeStyle = '#38bdf8';
        this.ctx.lineWidth = 2;
        roundRect(this.ctx, rx, ry, rw, rh, 3);
        this.ctx.stroke();
      }

      this.ctx.globalAlpha = 1.0;

      if (rw > 44 && rh > 22) {
        this.ctx.save();
        this.ctx.beginPath();
        this.ctx.rect(rx + 4, ry + 2, rw - 8, rh - 4);
        this.ctx.clip();

        this.ctx.shadowColor = 'rgba(0, 0, 0, 0.9)';
        this.ctx.shadowBlur = 4;
        this.ctx.fillStyle = '#ffffff';

        const label = shortLabel(node.item.name, isModuleLevel);
        const fontSize = rw > 120 && rh > 40 ? 12 : 11;
        this.ctx.font = `600 ${fontSize}px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif`;
        this.ctx.fillText(label, rx + 7, ry + 16, rw - 14);

        if (rh > 38 && rw > 50) {
          this.ctx.font = '500 10px "JetBrains Mono", monospace';
          const sizeLabel = this.formatBytes(this.sizeOf(node.item));
          this.ctx.fillText(sizeLabel, rx + 7, ry + 30, rw - 14);
        }

        if (hasDrill && rh > 52 && rw > 70) {
          this.ctx.font = '400 9px -apple-system, sans-serif';
          this.ctx.fillStyle = 'rgba(255,255,255,0.6)';
          this.ctx.fillText(`${node.item.children.length} modules ▸`, rx + 7, ry + 43, rw - 14);
        }

        this.ctx.restore();
      }

      this.ctx.restore();
    }
  }

  hitTest(x, y) {
    for (const node of this.nodes) {
      if (x >= node.x && x <= node.x + node.w && y >= node.y && y <= node.y + node.h) {
        return node;
      }
    }
    return null;
  }

  updateTooltip(node, clientX, clientY) {
    if (!node) {
      this.tooltip.classList.add('hidden');
      return;
    }
    const size = this.sizeOf(node.item);
    const parentTotal = this.currentLevel?.totalBytes || this.totalBytes;
    const pct = parentTotal > 0 ? ((size / parentTotal) * 100).toFixed(1) : '0.0';
    const escapedName = this.escapeHTML(node.item.name);
    const hasDrill = !!(node.item.children && node.item.children.length > 0);
    const isModuleLevel = this.drillStack.length > 0;

    let sizeLabel = this.useGzip ? 'Gzip' : 'Size';
    let extraLine = '';

    if (this.useGzip && node.item.sizeBytes) {
      extraLine = `<div class="text-[#8b949e] text-xs mt-0.5">Raw: ${this.formatBytes(node.item.sizeBytes)}</div>`;
    } else if (!this.useGzip && node.item.gzipBytes) {
      extraLine = `<div class="text-[#8b949e] text-xs mt-0.5">Gzip: ~${this.formatBytes(node.item.gzipBytes)}</div>`;
    }

    const chunkLine = node.item.chunks && node.item.chunks.length
      ? `<div class="text-[#6e7681] text-[10px] mt-1 truncate max-w-xs">${node.item.chunks.join(', ')}</div>`
      : (node.item.chunkIds && node.item.chunkIds.length
        ? `<div class="text-[#6e7681] text-[10px] mt-1 truncate max-w-xs">${node.item.chunkIds.join(', ')}</div>`
        : '');

    const drillHint = hasDrill
      ? `<div class="text-[#58a6ff] text-[10px] mt-1.5 font-semibold">Click to drill into ${node.item.children.length} modules</div>`
      : (isModuleLevel ? '' : '');

    this.tooltip.innerHTML = `
      <div class="font-bold text-[#e6edf3] text-[13px] mb-1">${escapedName}</div>
      <div class="text-[#8b949e] text-xs">${sizeLabel}: <span class="text-[#d29922] font-semibold">${this.formatBytes(size)}</span> <span class="text-[#6e7681]">(${pct}%)</span></div>
      ${extraLine}
      ${chunkLine}
      ${drillHint}
    `;

    const tt = this.tooltip;
    const ttRect = tt.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;

    let left = clientX + 14;
    let top = clientY + 14;
    if (left + 260 > vw) left = clientX - 260;
    if (top + 120 > vh) top = clientY - 120;

    tt.style.left = `${left}px`;
    tt.style.top = `${top}px`;
    tt.classList.remove('hidden');
  }

  escapeHTML(str) {
    return (str || '').replace(/[&<>"']/g, m => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    })[m]);
  }

  formatBytes(b) {
    if (!b) return '0 B';
    if (b >= 1048576) return (b / 1048576).toFixed(2) + ' MB';
    if (b >= 1024) return (b / 1024).toFixed(1) + ' KB';
    return b + ' B';
  }

  setSelected(name) {
    this.selectedName = name || '';
    this.draw();
  }

  setHighlight(query) {
    this.highlightQuery = (query || '').toLowerCase().trim();
    this.draw();
  }

  setGzipMode(useGzip) {
    this.useGzip = useGzip;
    if (this.currentLevel) {
      this.computeLayout();
      this.draw();
    }
  }
}

// Squarified treemap layout (Bruls, Huizing, van Wijk)
function squarify(items, x, y, w, h, sizeOf) {
  const total = items.reduce((s, i) => s + Math.max(0, sizeOf(i)), 0);
  if (total === 0 || w <= 0 || h <= 0) return [];

  const scaled = items
    .filter(i => sizeOf(i) > 0)
    .map(i => ({ item: i, area: (sizeOf(i) / total) * w * h }))
    .sort((a, b) => b.area - a.area);

  const nodes = [];
  layoutStrip(scaled, 0, x, y, w, h, nodes);
  return nodes;
}

function layoutStrip(items, start, x, y, w, h, nodes) {
  if (start >= items.length) return;
  if (items.length - start === 1) {
    nodes.push({ item: items[start].item, x, y, w, h });
    return;
  }

  const vertical = h <= w;
  const side = vertical ? h : w;
  let rowArea = 0;
  let best = Infinity;
  let end = start;

  for (let i = start; i < items.length; i++) {
    const testArea = rowArea + items[i].area;
    const worst = worstRatio(items, start, i + 1, testArea, side);
    if (worst <= best) {
      best = worst;
      end = i + 1;
      rowArea = testArea;
    } else {
      break;
    }
  }

  const rowSpan = rowArea / side;

  let offset = 0;
  for (let i = start; i < end; i++) {
    const len = items[i].area / rowSpan;
    if (vertical) {
      nodes.push({ item: items[i].item, x: x + offset, y, w: rowSpan, h: len });
      offset += len;
    } else {
      nodes.push({ item: items[i].item, x, y: y + offset, w: len, h: rowSpan });
      offset += len;
    }
  }

  if (vertical) {
    layoutStrip(items, end, x + rowSpan, y, w - rowSpan, h, nodes);
  } else {
    layoutStrip(items, end, x, y + rowSpan, w, h - rowSpan, nodes);
  }
}

function worstRatio(items, start, end, totalArea, side) {
  if (totalArea === 0 || side === 0) return Infinity;
  let worst = 0;
  const s2 = side * side;
  for (let i = start; i < end; i++) {
    const a = items[i].area;
    const r1 = (s2 * a) / (totalArea * totalArea);
    const r2 = (totalArea * totalArea) / (s2 * a);
    worst = Math.max(worst, Math.max(r1, r2));
  }
  return worst;
}

function shortLabel(name, isModule) {
  if (isModule) {
    const parts = name.split('/');
    if (parts.length > 2) {
      return '…/' + parts.slice(-2).join('/');
    }
    return name;
  }
  return name;
}

function shadeColor(hex, percent) {
  const num = parseInt(hex.replace('#', ''), 16);
  const r = Math.min(255, Math.max(0, ((num >> 16) & 0xff) + Math.round(2.55 * percent)));
  const g = Math.min(255, Math.max(0, ((num >> 8) & 0xff) + Math.round(2.55 * percent)));
  const b = Math.min(255, Math.max(0, (num & 0xff) + Math.round(2.55 * percent)));
  return `rgb(${r},${g},${b})`;
}

function roundRect(ctx, x, y, w, h, r) {
  if (w < 2 * r) r = w / 2;
  if (h < 2 * r) r = h / 2;
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}
