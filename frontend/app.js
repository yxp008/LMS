const API = '';
let currentPage = 1;
let charts = {};
let logAutoRefreshTimer = null;
let logAutoRefreshEnabled = true;

const LEVEL_MAP = { '1': 'INFO', '2': 'WARN', '3': 'ERROR', '4': 'DEBUG', 'emerg': 'EMERG', 'alert': 'ALERT', 'crit': 'CRIT', 'error': 'ERROR', 'warn': 'WARN', 'notice': 'NOTICE', 'info': 'INFO', 'debug': 'DEBUG' };
const LEVEL_COLOR = { 'INFO': '#3498db', 'WARN': '#f39c12', 'ERROR': '#e74c3c', 'DEBUG': '#2c3e50', 'EMERG': '#c0392b', 'ALERT': '#e74c3c', 'CRIT': '#e74c3c', 'NOTICE': '#2ecc71' };
const LEVEL_COLORS = ['#3498db','#f39c12','#e74c3c','#2c3e50','#2ecc71','#1abc9c','#e67e22','#34495e','#95a5a6'];
const CHANNEL_MAP = { '1': '邮件', '2': '短信', '3': 'Webhook' };
const ALERT_LEVEL_MAP = { '1': '严重', '2': '高', '3': '中', '4': '低' };
const STATUS_MAP = { '0': '停用', '1': '启用' };

document.addEventListener('DOMContentLoaded', () => {
    initNavigation();
    updateTime();
    setInterval(updateTime, 1000);
    loadDashboard();
    initFilterListeners();
    initDatePickers();
});

function initNavigation() {
    document.querySelectorAll('.nav-item').forEach(item => {
        item.addEventListener('click', () => {
            document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
            item.classList.add('active');

            const page = item.dataset.page;
            document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
            document.getElementById(`page-${page}`).classList.add('active');

            const titles = { logs: '日志查询', collectors: '采集器', alerts: '告警规则' };
            document.getElementById('page-title').textContent = titles[page] || '';

            if (page === 'logs') {
                stopLogAutoRefresh();
                const activeTab = document.querySelector('.sub-tab.active');
                const subtab = activeTab ? activeTab.dataset.subtab : 'dashboard';
                handleSubTabLoad(subtab);
            }
            if (page === 'collectors') { stopLogAutoRefresh(); loadCollectors(); }
            if (page === 'alerts') { stopLogAutoRefresh(); loadAlertRules(); }
        });
    });
}

function updateTime() {
    document.getElementById('current-time').textContent = new Date().toLocaleString('zh-CN');
}

// ========== Sub Tabs ==========
let currentSubTab = 'dashboard';

async function switchToLogsWithLevel(level) {
    document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
    document.querySelector('.nav-item[data-page="logs"]').classList.add('active');
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.getElementById('page-logs').classList.add('active');
    document.getElementById('page-title').textContent = '日志查询';
    switchSubTab('logs');
    // 等待筛选选项加载完成后再设置值和查询
    await loadFilterOptions();
    document.getElementById('filter-level').value = level;
    loadLogs(1);
}

function switchSubTab(subtab) {
    stopLogAutoRefresh();
    currentSubTab = subtab;

    document.querySelectorAll('.sub-tab').forEach(t => t.classList.remove('active'));
    document.querySelector(`.sub-tab[data-subtab="${subtab}"]`).classList.add('active');

    document.querySelectorAll('.sub-content').forEach(c => c.classList.remove('active'));
    document.getElementById(`subtab-${subtab}`).classList.add('active');

    handleSubTabLoad(subtab);
}

function handleSubTabLoad(subtab) {
    if (subtab === 'dashboard') {
        loadDashboard();
    } else if (subtab === 'logs') {
        loadFilterOptions();
        loadLogs(1);
        startLogAutoRefresh();
    } else if (subtab === 'charts') {
        loadCharts();
    }
}

async function fetchAPI(url) {
    try {
        const resp = await fetch(`${API}${url}`);
        return await resp.json();
    } catch (e) {
        console.error('API error:', e);
        return null;
    }
}

async function postAPI(url, data) {
    try {
        const resp = await fetch(`${API}${url}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        return await resp.json();
    } catch (e) {
        console.error('API error:', e);
        return null;
    }
}

// ========== Dashboard ==========
async function loadDashboard() {
    const stats = await fetchAPI('/api/stats');
    if (!stats) return;

    document.getElementById('stat-total').textContent = formatNum(stats.total_logs);
    document.getElementById('stat-errors').textContent = formatNum(stats.error_count);
    document.getElementById('stat-warns').textContent = formatNum(stats.warn_count);
    document.getElementById('stat-24h').textContent = formatNum(stats.last_24h);

    const [levels, sources] = await Promise.all([
        fetchAPI('/api/levels'),
        fetchAPI('/api/sources')
    ]);

    renderLevelChart(levels || []);
    renderSourceChart(sources || []);
    loadRecentErrors();
}

async function loadRecentErrors() {
    const data = await fetchAPI('/api/logs?page=1&page_size=10&level=3');
    const container = document.getElementById('recent-errors');

    if (!data || !data.data || data.data.length === 0) {
        container.innerHTML = '<div class="empty-state">暂无错误日志</div>';
        return;
    }

    container.innerHTML = data.data.map(log => `
        <div class="log-item" onclick='showLogDetail(${JSON.stringify(log)})'>
            <span class="log-time">${formatTime(log.Timestamp)}</span>
            <span class="level-badge level-3">ERROR</span>
            <span class="log-msg">${escapeHtml(log.Message)}</span>
        </div>
    `).join('');
}

function destroyChart(key) {
    if (charts[key]) {
        charts[key].destroy();
        charts[key] = null;
    }
}

function renderLevelChart(data) {
    destroyChart('levels');
    const ctx = document.getElementById('chart-levels');
    if (!ctx) return;

    // 归一化 Level 名称并合并
    const merged = {};
    data.forEach(d => {
        const lv = LEVEL_MAP[d.Level] || d.Level;
        merged[lv] = (merged[lv] || 0) + parseInt(d.count);
    });
    const labels = Object.keys(merged);
    const values = labels.map(l => merged[l]);
    const colors = labels.map((l, i) => LEVEL_COLOR[l] || ['#3498db','#e74c3c','#f39c12','#2ecc71','#9b59b6','#1abc9c'][i % 6]);

    charts['levels'] = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels,
            datasets: [{ data: values, backgroundColor: colors, borderWidth: 0 }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'right',
                    labels: { color: '#8899a6', padding: 16, font: { size: 13 } }
                }
            },
            cutout: '65%'
        }
    });
}

function renderSourceChart(data) {
    destroyChart('sources');
    const ctx = document.getElementById('chart-sources');
    if (!ctx) return;

    const colors = ['#1da1f2', '#2ecc71', '#f39c12', '#e74c3c', '#9b59b6', '#1abc9c', '#e67e22', '#34495e'];

    charts['sources'] = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: data.map(d => d.Source_Type || '未知'),
            datasets: [{ data: data.map(d => parseInt(d.count)), backgroundColor: colors.slice(0, data.length), borderWidth: 0 }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'right',
                    labels: { color: '#8899a6', padding: 16, font: { size: 13 } }
                }
            },
            cutout: '65%'
        }
    });
}

// ========== Logs ==========
async function loadFilterOptions() {
    const [levels, sources] = await Promise.all([
        fetchAPI('/api/levels'),
        fetchAPI('/api/sources')
    ]);
    const levelSelect = document.getElementById('filter-level');
    levelSelect.innerHTML = '<option value="">所有级别</option>' +
        (levels || []).map(l => `<option value="${escapeHtml(l.Level)}">${escapeHtml(LEVEL_MAP[l.Level] || l.Level)} (${l.count})</option>`).join('');
    (levels || []).forEach((l, i) => {
        if (!LEVEL_MAP[l.Level]) LEVEL_MAP[l.Level] = l.Level;
        if (!LEVEL_COLOR[l.Level]) LEVEL_COLOR[l.Level] = LEVEL_COLORS[i % LEVEL_COLORS.length];
    });

    const sourceSelect = document.getElementById('filter-source');
    sourceSelect.innerHTML = '<option value="">所有来源</option>' +
        (sources || []).map(s => `<option value="${escapeHtml(s.Source_Type)}">${escapeHtml(s.Source_Type)} (${s.count})</option>`).join('');
}

function initFilterListeners() {
    document.getElementById('filter-search').addEventListener('keydown', e => {
        if (e.key === 'Enter') loadLogs(1);
    });
    document.getElementById('filter-host').addEventListener('keydown', e => {
        if (e.key === 'Enter') loadLogs(1);
    });
}

// ========== 自定义日历选择器 ==========
let dpInput = null, dpYear = 0, dpMonth = 0;

function initDatePickers() {
    const dp = document.createElement('div');
    dp.id = 'datepicker-popup';
    dp.className = 'datepicker-popup';
    dp.innerHTML = '<div class="dp-header"><button class="dp-nav" data-dir="-1">◀</button><span class="dp-title"></span><button class="dp-nav" data-dir="1">▶</button></div><div class="dp-week"><span>日</span><span>一</span><span>二</span><span>三</span><span>四</span><span>五</span><span>六</span></div><div class="dp-grid"></div><div class="dp-footer"><button class="dp-clear">清除</button></div>';
    document.body.appendChild(dp);

    dp.querySelector('.dp-nav[data-dir="-1"]').onclick = () => navMonth(-1);
    dp.querySelector('.dp-nav[data-dir="1"]').onclick = () => navMonth(1);
    dp.querySelector('.dp-clear').onclick = () => { if (dpInput) { dpInput.value = ''; } hideDP(); };

    document.getElementById('filter-start').addEventListener('click', e => showDP(e.target));
    document.getElementById('filter-end').addEventListener('click', e => showDP(e.target));
    document.addEventListener('click', e => { if (!e.target.closest('.datepicker-popup') && !e.target.matches('#filter-start,#filter-end')) hideDP(); });
}

function showDP(input) {
    dpInput = input;
    const val = input.value;
    if (val && /^\d{4}-\d{2}-\d{2}$/.test(val)) {
        const [y, m] = val.split('-').map(Number);
        dpYear = y; dpMonth = m - 1;
    } else {
        const now = new Date();
        dpYear = now.getFullYear(); dpMonth = now.getMonth();
    }
    renderCalendar();
    const rect = input.getBoundingClientRect();
    const popup = document.getElementById('datepicker-popup');
    popup.style.top = (rect.bottom + window.scrollY + 4) + 'px';
    popup.style.left = (rect.left + window.scrollX) + 'px';
    popup.classList.add('show');
}

function hideDP() {
    document.getElementById('datepicker-popup').classList.remove('show');
    dpInput = null;
}

function navMonth(dir) {
    dpMonth += dir;
    if (dpMonth < 0) { dpMonth = 11; dpYear--; }
    if (dpMonth > 11) { dpMonth = 0; dpYear++; }
    renderCalendar();
}

function renderCalendar() {
    const popup = document.getElementById('datepicker-popup');
    popup.querySelector('.dp-title').textContent = dpYear + '年 ' + (dpMonth + 1) + '月';
    const grid = popup.querySelector('.dp-grid');
    const firstDay = new Date(dpYear, dpMonth, 1).getDay();
    const daysInMonth = new Date(dpYear, dpMonth + 1, 0).getDate();
    const today = new Date();
    const todayStr = today.getFullYear()+'-'+String(today.getMonth()+1).padStart(2,'0')+'-'+String(today.getDate()).padStart(2,'0');
    const selected = dpInput ? dpInput.value : '';

    let html = '';
    for (let i = 0; i < firstDay; i++) html += '<span></span>';
    for (let d = 1; d <= daysInMonth; d++) {
        const ds = dpYear+'-'+String(dpMonth+1).padStart(2,'0')+'-'+String(d).padStart(2,'0');
        let cls = '';
        if (ds === todayStr) cls += ' today';
        if (ds === selected) cls += ' selected';
        html += `<span class="${cls}" data-date="${ds}">${d}</span>`;
    }
    grid.innerHTML = html;
    grid.querySelectorAll('span[data-date]').forEach(el => {
        el.onclick = () => {
            if (dpInput) {
                dpInput.value = el.dataset.date;
                console.log('[DatePicker] set', dpInput.id, '=', dpInput.value);
            }
            hideDP();
            // 日期选择后自动触发查询
            const startVal = document.getElementById('filter-start').value;
            const endVal = document.getElementById('filter-end').value;
            if (startVal || endVal) loadLogs(1);
        };
    });
}

function resetFilters() {
    document.getElementById('filter-level').value = '';
    document.getElementById('filter-host').value = '';  // 文本输入
    document.getElementById('filter-source').value = '';
    document.getElementById('filter-search').value = '';
    document.getElementById('filter-start').value = '';
    document.getElementById('filter-end').value = '';
    loadLogs(1);
}

async function loadLogs(page) {
    currentPage = page;
    const params = new URLSearchParams();
    params.set('page', page);
    params.set('page_size', '50');

    const level = document.getElementById('filter-level').value;
    const host = document.getElementById('filter-host').value.trim();
    const source = document.getElementById('filter-source').value;
    const search = document.getElementById('filter-search').value;
    const startDate = document.getElementById('filter-start').value;
    const endDate = document.getElementById('filter-end').value;

    if (level) params.set('level', level);
    if (host) params.set('host', host);
    if (source) params.set('source_type', source);
    if (search) params.set('search', search);
    if (startDate) params.set('start_time', startDate + ' 00:00:00');
    if (endDate) params.set('end_time', endDate + ' 23:59:59');

    const url = `/api/logs?${params}`;
    console.log('[loadLogs]', url);
    const data = await fetchAPI(url);
    if (!data) { console.log('[loadLogs] fetch returned null'); return; }
    console.log('[loadLogs] total:', data.total, 'data length:', data.data ? data.data.length : 0);

    const tbody = document.getElementById('logs-table-body');

    if (!data.data || data.data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="empty-state">暂无匹配的日志数据</td></tr>';
        document.getElementById('result-count').textContent = '';
        document.getElementById('pagination').innerHTML = '';
        return;
    }

    document.getElementById('result-count').textContent =
        `查询结果：共 ${data.total.toLocaleString()} 条，当前第 ${data.page} 页`;
    tbody.innerHTML = data.data.map(log => {
        const tags = log.Tags || {};
        const tagStr = Object.keys(tags).length > 0
            ? Object.entries(tags).map(([k, v]) => `<span class="tag-item">${escapeHtml(k)}: ${escapeHtml(v)}</span>`).join(' ')
            : '<span style="color:var(--text-secondary)">-</span>';
        const level = String(log.Level);
        return `
        <tr class="clickable" onclick='showLogDetail(${JSON.stringify(log).replace(/'/g, "&#39;")})'>
            <td>${formatTime(log.Timestamp)}</td>
            <td><span style="display:inline-block;padding:2px 8px;border-radius:12px;font-size:12px;font-weight:600;background:${LEVEL_COLOR[level] || '#555'}22;color:${LEVEL_COLOR[level] || '#aaa'}">${LEVEL_MAP[level] || level}</span></td>
            <td title="${escapeHtml(log.Host)}">${escapeHtml(log.Host)}</td>
            <td title="${escapeHtml(log.Source_Type)}">${escapeHtml(log.Source_Type)}</td>
            <td title="${escapeHtml(log.Message)}">${escapeHtml(log.Message)}</td>
            <td class="tags-cell">${tagStr}</td>
            <td>${escapeHtml(log.Collector_ID || '')}</td>
        </tr>`;
    }).join('');

    renderPagination(data.page, data.total_pages, data.total);
}

function renderPagination(page, totalPages, total) {
    const container = document.getElementById('pagination');
    if (totalPages <= 1) {
        container.innerHTML = `<span class="page-info">共 ${formatNum(total)} 条记录</span>`;
        return;
    }

    let html = `<button ${page <= 1 ? 'disabled' : ''} onclick="loadLogs(${page - 1})">上一页</button>`;

    const start = Math.max(1, page - 2);
    const end = Math.min(totalPages, page + 2);

    if (start > 1) html += `<button onclick="loadLogs(1)">1</button><span class="page-info">...</span>`;

    for (let i = start; i <= end; i++) {
        html += `<button class="${i === page ? 'active' : ''}" onclick="loadLogs(${i})">${i}</button>`;
    }

    if (end < totalPages) html += `<span class="page-info">...</span><button onclick="loadLogs(${totalPages})">${totalPages}</button>`;

    html += `<button ${page >= totalPages ? 'disabled' : ''} onclick="loadLogs(${page + 1})">下一页</button>`;
    html += `<span class="page-info">共 ${formatNum(total)} 条</span>`;
    html += `<span class="page-jump">跳至 <input type="number" id="jump-page-input" min="1" max="${totalPages}" value="${page}" onkeydown="if(event.key==='Enter')jumpToPage(${totalPages})" style="width:50px;text-align:center"> 页 <button class="btn btn-sm" onclick="jumpToPage(${totalPages})">GO</button></span>`;

    container.innerHTML = html;
}

function jumpToPage(totalPages) {
    const input = document.getElementById('jump-page-input');
    const page = parseInt(input.value);
    if (isNaN(page) || page < 1 || page > totalPages) {
        input.value = currentPage;
        return;
    }
    loadLogs(page);
}

// ========== Charts ==========
async function loadCharts() {
    destroyChart('timeline');
    destroyChart('hosts');
    destroyChart('sourcePie');

    const [timeline, hostsDetail, sources] = await Promise.all([
        fetchAPI('/api/timeline?hours=24'),
        fetchAPI('/api/hosts?detail=1'),
        fetchAPI('/api/sources')
    ]);

    renderTimeline(timeline || []);
    renderHostChart(hostsDetail || []);
    renderSourcePie(sources || []);
}

function renderTimeline(data) {
    const ctx = document.getElementById('chart-timeline');
    if (!ctx) return;

    const timeMap = {};
    data.forEach(d => {
        const t = d.time_point;
        const lv = LEVEL_MAP[d.Level] || d.Level;
        if (!timeMap[t]) timeMap[t] = {};
        timeMap[t][lv] = (timeMap[t][lv] || 0) + parseInt(d.count);
    });

    const times = Object.keys(timeMap).sort();
    // 动态获取实际存在的所有 Level（归一化后）
    const allLevels = [...new Set(data.map(d => LEVEL_MAP[d.Level] || d.Level))].sort();
    const datasets = allLevels.map(level => ({
        label: level,
        data: times.map(t => timeMap[t][level] || 0),
        backgroundColor: LEVEL_COLOR[level],
        borderColor: LEVEL_COLOR[level],
        borderWidth: 2,
        fill: false,
        tension: 0.3,
        pointRadius: 2
    }));

    charts['timeline'] = new Chart(ctx, {
        type: 'line',
        data: { labels: times.map((t, i) => formatTimeShort(t, times[i + 1])), datasets },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: { mode: 'index', intersect: false },
            scales: {
                x: { ticks: { color: '#8899a6', maxTicksLimit: 12 }, grid: { color: 'rgba(42,58,74,0.5)' } },
                y: { ticks: { color: '#8899a6' }, grid: { color: 'rgba(42,58,74,0.5)' }, beginAtZero: true }
            },
            plugins: {
                legend: { labels: { color: '#8899a6', font: { size: 12 } } }
            }
        }
    });
}

function renderHostChart(data) {
    const ctx = document.getElementById('chart-hosts');
    if (!ctx) return;

    // 按主机聚合每个等级的计数（归一化 Level 名称）
    const hostLevels = {};
    const allLevels = new Set();
    data.forEach(d => {
        const lv = LEVEL_MAP[d.Level] || d.Level;
        if (!hostLevels[d.Host]) hostLevels[d.Host] = {};
        hostLevels[d.Host][lv] = (hostLevels[d.Host][lv] || 0) + parseInt(d.count);
        allLevels.add(lv);
    });
    // 取前 10 主机
    const topHosts = Object.entries(hostLevels)
        .map(([host, levels]) => ({ host, total: Object.values(levels).reduce((a, b) => a + b, 0) }))
        .sort((a, b) => b.total - a.total).slice(0, 10).map(h => h.host);

    const sortedLevels = [...allLevels].sort();
    const colors = ['#3498db', '#f39c12', '#e74c3c', '#9b59b6', '#2ecc71', '#1abc9c', '#e67e22', '#34495e'];
    const datasets = sortedLevels.map((lv, i) => ({
        label: LEVEL_MAP[lv] || lv,
        data: topHosts.map(h => hostLevels[h] ? (hostLevels[h][lv] || 0) : 0),
        backgroundColor: LEVEL_COLOR[lv] || colors[i % colors.length],
        borderRadius: 0
    }));

    charts['hosts'] = new Chart(ctx, {
        type: 'bar',
        data: { labels: topHosts, datasets },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            indexAxis: 'y',
            plugins: {
                legend: { labels: { color: '#8899a6', font: { size: 11 } } },
                tooltip: { mode: 'index', intersect: false }
            },
            scales: {
                x: { stacked: true, ticks: { color: '#8899a6' }, grid: { color: 'rgba(42,58,74,0.5)' } },
                y: { stacked: true, ticks: { color: '#8899a6', font: { size: 11 } }, grid: { display: false } }
            }
        }
    });
}

function renderSourcePie(data) {
    const ctx = document.getElementById('chart-source-pie');
    if (!ctx) return;

    const colors = ['#1da1f2', '#2ecc71', '#f39c12', '#e74c3c', '#9b59b6', '#1abc9c', '#e67e22', '#34495e'];

    charts['sourcePie'] = new Chart(ctx, {
        type: 'pie',
        data: {
            labels: data.map(d => d.Source_Type || '未知'),
            datasets: [{
                data: data.map(d => parseInt(d.count)),
                backgroundColor: colors.slice(0, data.length),
                borderWidth: 0
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'right',
                    labels: { color: '#8899a6', padding: 12, font: { size: 12 } }
                }
            }
        }
    });
}

// ========== Collectors ==========
async function loadCollectors() {
    const data = await fetchAPI('/api/collectors');
    const tbody = document.getElementById('collectors-table-body');

    if (!data || data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" class="empty-state">暂无采集器数据</td></tr>';
        return;
    }

    tbody.innerHTML = data.map(c => {
        const enabled = c.Status === '1';
        const sourceTypes = (c.Source_Types || []).map(s =>
            `<span class="source-tag ${s.enabled ? 'source-on' : 'source-off'}" onclick="toggleSourceType('${escapeHtml(s.key)}', ${!s.enabled})" title="点击${s.enabled ? '停用' : '启用'}${escapeHtml(s.name)}">${escapeHtml(s.name)}</span>`
        ).join(' ');
        return `
        <tr>
            <td>${escapeHtml(c.Collector_ID)}</td>
            <td>${escapeHtml(c.Name)}</td>
            <td>${sourceTypes || '<span style="color:var(--text-secondary)">-</span>'}</td>
            <td><span class="level-badge ${enabled ? 'level-1' : 'level-3'}">${enabled ? '已启用' : '已停用'}</span></td>
            <td><button class="btn btn-sm ${enabled ? 'btn-danger' : 'btn-primary'}" onclick="toggleCollector('${escapeHtml(c.Collector_ID)}', '${enabled ? '0' : '1'}')">${enabled ? '停用' : '启用'}</button></td>
        </tr>`;
    }).join('');
}

function toggleSourceType(type, enable) {
    const currentPrefs = {};
    const tags = document.querySelectorAll('.source-tag');
    tags.forEach(t => {
        const cls = t.className;
        currentPrefs[t.getAttribute('data-key') || t.textContent] = cls.includes('source-on');
    });
    // 从 API 获取当前状态再操作
    fetchAPI('/api/collection-prefs').then(prefs => {
        if (!prefs) return;
        prefs[type] = enable;
        const text = enable ? '正在启用采集源...' : '正在停用采集源...';
        document.getElementById('collector-loading-text').textContent = text;
        document.getElementById('collector-loading-modal').classList.remove('hidden');
        postAPI('/api/collection-prefs', prefs).then(result => {
            setTimeout(() => {
                document.getElementById('collector-loading-modal').classList.add('hidden');
                loadCollectors();
            }, 800);
        });
    });
}

function toggleCollector(id, newStatus) {
    const text = newStatus === '0' ? '采集器停用中...' : '采集器启用中...';
    document.getElementById('collector-loading-text').textContent = text;
    document.getElementById('collector-loading-modal').classList.remove('hidden');
    postAPI('/api/collectors', { action: 'update_status', Collector_ID: id, Status: newStatus }).then(result => {
        setTimeout(() => {
            document.getElementById('collector-loading-modal').classList.add('hidden');
            loadCollectors();
        }, 800);
    });
}

// ========== Alert Rules ==========
async function loadAlertRules() {
    const data = await fetchAPI('/api/alert-rules');
    const tbody = document.getElementById('alerts-table-body');

    if (!data || data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="11" class="empty-state">暂无告警规则</td></tr>';
        return;
    }

    tbody.innerHTML = data.map(r => `
        <tr>
            <td>${escapeHtml(r.AlertRule_ID)}</td>
            <td>${escapeHtml(r.Name)}</td>
            <td title="${escapeHtml(r.Desc)}">${escapeHtml(r.Desc)}</td>
            <td><code style="font-size:11px">${escapeHtml(r.Alert_Sql)}</code></td>
            <td>${escapeHtml(r.Interval)}</td>
            <td>${CHANNEL_MAP[r.Channel] || r.Channel}</td>
            <td>${escapeHtml(r.Address)}</td>
            <td>${ALERT_LEVEL_MAP[r.Level] || r.Level}</td>
            <td><span class="level-badge ${r.Status === '1' ? 'level-1' : 'level-3'}">${STATUS_MAP[r.Status] || r.Status}</span></td>
            <td>${formatTime(r.Created_Time)}</td>
            <td>
                <button class="btn btn-sm" onclick='editAlertRule(${JSON.stringify(r).replace(/'/g, "&#39;")})'>编辑</button>
                <button class="btn btn-sm btn-danger" onclick="deleteAlertRule('${escapeHtml(r.AlertRule_ID)}')" style="margin-left:4px">删除</button>
            </td>
        </tr>
    `).join('');
}

function showAddAlertRule() {
    document.getElementById('alert-rule-modal-title').textContent = '新建告警规则';
    document.getElementById('alert-edit-id').value = '';
    document.getElementById('alert-name').value = '';
    document.getElementById('alert-desc').value = '';
    document.getElementById('alert-sql').value = '';
    document.getElementById('alert-interval').value = '';
    document.getElementById('alert-channel').value = '1';
    document.getElementById('alert-address').value = '';
    document.getElementById('alert-level').value = '3';
    document.getElementById('alert-status').value = '1';
    document.getElementById('alert-rule-modal').classList.remove('hidden');
}

function editAlertRule(rule) {
    document.getElementById('alert-rule-modal-title').textContent = '编辑告警规则';
    document.getElementById('alert-edit-id').value = rule.AlertRule_ID;
    document.getElementById('alert-name').value = rule.Name || '';
    document.getElementById('alert-desc').value = rule.Desc || '';
    document.getElementById('alert-sql').value = rule.Alert_Sql || '';
    document.getElementById('alert-interval').value = rule.Interval || '';
    document.getElementById('alert-channel').value = rule.Channel || '1';
    document.getElementById('alert-address').value = rule.Address || '';
    document.getElementById('alert-level').value = rule.Level || '3';
    document.getElementById('alert-status').value = rule.Status || '1';
    document.getElementById('alert-rule-modal').classList.remove('hidden');
}

function closeAlertRuleModal() {
    document.getElementById('alert-rule-modal').classList.add('hidden');
    document.getElementById('sql-validation-result').style.display = 'none';
}

async function saveAlertRule() {
    const editId = document.getElementById('alert-edit-id').value;
    const name = document.getElementById('alert-name').value.trim();
    const desc = document.getElementById('alert-desc').value.trim();
    const alertSql = document.getElementById('alert-sql').value.trim();
    const interval = document.getElementById('alert-interval').value.trim();
    const channel = document.getElementById('alert-channel').value;
    const address = document.getElementById('alert-address').value.trim();
    const level = document.getElementById('alert-level').value;
    const status = document.getElementById('alert-status').value;

    if (!name || !alertSql || !interval || !address) {
        alert('请填写所有必填字段');
        return;
    }

    const result = await postAPI('/api/alert-rules', {
        action: editId ? 'update' : 'create',
        AlertRule_ID: editId,
        Name: name, Desc: desc, Alert_Sql: alertSql,
        Interval: interval, Channel: channel, Address: address,
        Level: level, Status: status
    });

    if (result && result.ok) {
        closeAlertRuleModal();
        loadAlertRules();
    } else {
        alert('保存失败: ' + (result && result.error || '未知错误'));
    }
}

async function deleteAlertRule(id) {
    if (!confirm('确定删除此告警规则？')) return;
    const result = await postAPI('/api/alert-rules', { action: 'delete', AlertRule_ID: id });
    if (result && result.ok) {
        loadAlertRules();
    } else {
        alert('删除失败: ' + (result && result.error || '未知错误'));
    }
}

// ========== Log Auto Refresh ==========
function startLogAutoRefresh() {
    stopLogAutoRefresh();
    logAutoRefreshTimer = setInterval(() => {
        if (logAutoRefreshEnabled) {
            loadLogs(currentPage);
        }
    }, 5000);
}

function stopLogAutoRefresh() {
    if (logAutoRefreshTimer) {
        clearInterval(logAutoRefreshTimer);
        logAutoRefreshTimer = null;
    }
}

function toggleLogAutoRefresh() {
    logAutoRefreshEnabled = !logAutoRefreshEnabled;
    const btn = document.getElementById('btn-auto-refresh');
    if (btn) {
        btn.textContent = logAutoRefreshEnabled ? '自动刷新: 开' : '自动刷新: 关';
        btn.className = logAutoRefreshEnabled ? 'btn btn-primary btn-sm' : 'btn btn-sm';
    }
}

// ========== Log Detail Modal ==========
function showLogDetail(log) {
    const tags = log.Tags || {};
    const tagsHtml = Object.keys(tags).length > 0
        ? Object.entries(tags).map(([k, v]) => `<div class="detail-row"><div class="detail-label">${escapeHtml(k)}</div><div class="detail-value">${escapeHtml(v)}</div></div>`).join('')
        : '<div style="color:var(--text-secondary)">无额外标签</div>';

    document.getElementById('log-detail-body').innerHTML = `
        <div class="detail-row"><div class="detail-label">日志ID</div><div class="detail-value">${escapeHtml(log.Log_ID)}</div></div>
        <div class="detail-row"><div class="detail-label">时间</div><div class="detail-value">${formatTime(log.Timestamp)}</div></div>
        <div class="detail-row"><div class="detail-label">级别</div><div class="detail-value"><span class="level-badge level-${String(log.Level)}">${LEVEL_MAP[String(log.Level)] || log.Level}</span></div></div>
        <div class="detail-row"><div class="detail-label">主机</div><div class="detail-value">${escapeHtml(log.Host)}</div></div>
        <div class="detail-row"><div class="detail-label">来源类型</div><div class="detail-value">${escapeHtml(log.Source_Type)}</div></div>
        <div class="detail-row"><div class="detail-label">采集器</div><div class="detail-value">${escapeHtml(log.Collector_ID)}</div></div>
        <div class="detail-row"><div class="detail-label">消息内容</div><div class="detail-value"><pre>${escapeHtml(log.Message)}</pre></div></div>
        <div class="detail-row"><div class="detail-label">扩展标签</div><div class="detail-value">${tagsHtml}</div></div>
    `;

    document.getElementById('log-detail-modal').classList.remove('hidden');
}

function closeModal() {
    document.getElementById('log-detail-modal').classList.add('hidden');
}

document.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
        closeModal();
        closeAlertRuleModal();
        closeSmtpConfig();
        closeAlertTriggers();
    }
});

document.getElementById('log-detail-modal').addEventListener('click', e => {
    if (e.target === e.currentTarget) closeModal();
});

// ========== SQL Validation ==========
async function validateSql() {
    const sql = document.getElementById('alert-sql').value.trim();
    const resultDiv = document.getElementById('sql-validation-result');
    resultDiv.style.display = 'block';

    if (!sql) {
        resultDiv.innerHTML = '<span style="color:var(--error);font-size:12px">请输入SQL语句</span>';
        return;
    }

    resultDiv.innerHTML = '<span style="color:var(--accent);font-size:12px">正在验证...</span>';
    const result = await postAPI('/api/sql-validate', { sql });

    if (result && result.ok) {
        resultDiv.innerHTML = '<span style="color:var(--success);font-size:12px">&#10003; ' + (result.message || 'SQL语句语法正确') + '</span>';
    } else {
        resultDiv.innerHTML = '<span style="color:var(--error);font-size:12px">&#10007; ' + (result ? result.error : '验证失败') + '</span>';
    }
}

// ========== Alert Triggers ==========
async function showAlertTriggers() {
    document.getElementById('alert-triggers-modal').classList.remove('hidden');
    const data = await fetchAPI('/api/alert-triggers');
    const tbody = document.getElementById('alert-triggers-body');

    if (!data || data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="empty-state">暂无告警规则</td></tr>';
        return;
    }

    const channelMap = {'1': '邮件', '2': '短信', '3': 'Webhook'};
    const levelMap = {'1': '严重', '2': '高', '3': '中', '4': '低'};
    const statusMap = {'0': '停用', '1': '启用'};

    tbody.innerHTML = data.map(t => {
        const tc = parseInt(t.Trigger_Count) || 0;
        const countClass = tc > 0 ? 'level-3' : 'level-1';
        const statusClass = t.Status === '1' ? 'level-1' : 'level-3';
        return `
        <tr>
            <td>${escapeHtml(t.AlertRule_ID)}</td>
            <td>${escapeHtml(t.Rule_Name)}</td>
            <td title="${escapeHtml(t.Desc)}" style="max-width:180px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${escapeHtml(t.Desc || '-')}</td>
            <td>${channelMap[t.Channel] || t.Channel}</td>
            <td title="${escapeHtml(t.Address)}" style="max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${escapeHtml(t.Address)}</td>
            <td>${levelMap[t.Level] || t.Level}</td>
            <td><span class="level-badge ${countClass}">${tc} 次</span></td>
            <td>${t.Latest_Time ? formatTime(t.Latest_Time) : '<span style="color:var(--text-secondary)">-</span>'}</td>
        </tr>`;
    }).join('');
}

function closeAlertTriggers() {
    document.getElementById('alert-triggers-modal').classList.add('hidden');
}

// ========== SMTP Config ==========
function showSmtpConfig() {
    fetchAPI('/api/smtp-config').then(cfg => {
        if (cfg) {
            document.getElementById('smtp-host').value = cfg.host || '';
            document.getElementById('smtp-port').value = cfg.port || 465;
            document.getElementById('smtp-sender').value = cfg.sender || '';
            document.getElementById('smtp-password').value = cfg.password || '';
            if (cfg.host) {
                document.getElementById('smtp-status').textContent = '已配置';
                document.getElementById('smtp-status').style.color = '#2ecc71';
            }
        }
    });
    document.getElementById('smtp-config-modal').classList.remove('hidden');
}

function closeSmtpConfig() {
    document.getElementById('smtp-config-modal').classList.add('hidden');
}

async function saveSmtpConfig() {
    const config = {
        host: document.getElementById('smtp-host').value.trim(),
        port: document.getElementById('smtp-port').value,
        sender: document.getElementById('smtp-sender').value.trim(),
        password: document.getElementById('smtp-password').value,
    };
    if (!config.host || !config.sender || !config.password) {
        alert('请填写所有必填字段');
        return;
    }
    const result = await postAPI('/api/smtp-config', config);
    if (result && result.ok) {
        document.getElementById('smtp-status').textContent = '已配置';
        document.getElementById('smtp-status').style.color = '#2ecc71';
        closeSmtpConfig();
    } else {
        alert('保存失败');
    }
}

async function testSmtpConnection() {
    const resultDiv = document.getElementById('smtp-test-result');
    resultDiv.style.display = 'block';
    resultDiv.innerHTML = '<span style="color:var(--accent)">正在测试连接...</span>';

    const config = {
        host: document.getElementById('smtp-host').value.trim(),
        port: document.getElementById('smtp-port').value,
        sender: document.getElementById('smtp-sender').value.trim(),
        password: document.getElementById('smtp-password').value,
    };

    if (!config.host || !config.sender || !config.password) {
        resultDiv.innerHTML = '<span style="color:var(--error)">请先填写所有必填字段</span>';
        return;
    }

    const result = await postAPI('/api/smtp-test', config);
    if (result && result.ok) {
        resultDiv.innerHTML = '<span style="color:var(--success)">连接成功！SMTP服务器可正常访问</span>';
    } else {
        resultDiv.innerHTML = `<span style="color:var(--error)">连接失败: ${result ? result.error : '未知错误'}</span>`;
    }
}

// ========== Utilities ==========
function formatNum(n) {
    if (n === null || n === undefined) return '0';
    n = parseInt(n);
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return n.toString();
}

function formatTime(ts) {
    if (!ts) return '-';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return ts;
    const pad = n => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function formatTimeShort(ts, nextTs) {
    if (!ts) return '-';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return ts;
    const pad = n => String(n).padStart(2, '0');
    // 计算时间间隔
    let label = `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
    if (nextTs) {
        const next = new Date(nextTs);
        if (!isNaN(next.getTime())) {
            label += `~${pad(next.getHours())}:${pad(next.getMinutes())}`;
        }
    }
    return label;
}

function escapeHtml(str) {
    if (str === null || str === undefined) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}
