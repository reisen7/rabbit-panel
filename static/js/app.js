/**
 * 主应用模块
 */

let systemStatsInterval = null;
let containersInterval = null;

// 加载系统监控数据
async function loadSystemStats() {
    try {
        const response = await authFetch('/api/system/stats');
        if (!response.ok) throw new Error('获取系统信息失败');
        const data = await response.json();
        
        DOM.get('cpu-usage').textContent = data.cpu.toFixed(1) + '%';
        DOM.get('memory-usage').textContent = data.memory.toFixed(1) + '%';
        DOM.get('disk-usage').textContent = data.disk.toFixed(1) + '%';
        DOM.get('current-time').textContent = data.time;
    } catch (error) {
        console.error('加载系统监控失败:', error);
    }
}

// 显示主页面
function showMainPage() {
    DOM.get('login-page').classList.add('hidden');
    DOM.get('main-page').classList.remove('hidden');
    loadSystemStats();
    loadContainers();
    loadImages();
    loadComposeProjects();
    startIntervals();
}

// 启动定时器
function startIntervals() {
    if (!systemStatsInterval) {
        systemStatsInterval = setInterval(loadSystemStats, 5000);
    }
    startContainersAutoRefresh();
}

// 停止所有定时器
function stopAllIntervals() {
    if (systemStatsInterval) {
        clearInterval(systemStatsInterval);
        systemStatsInterval = null;
    }
    stopContainersAutoRefresh();
}

// 容器自动刷新
function startContainersAutoRefresh() {
    if (containersInterval) return;
    containersInterval = setInterval(loadContainers, 5000);
}

function stopContainersAutoRefresh() {
    if (containersInterval) {
        clearInterval(containersInterval);
        containersInterval = null;
    }
}

// 标签页切换
function initTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            const tab = this.dataset.tab;
            
            document.querySelectorAll('.tab-btn').forEach(b => {
                b.classList.remove('border-blue-500', 'text-blue-600');
                b.classList.add('border-transparent', 'text-gray-500');
            });
            this.classList.remove('border-transparent', 'text-gray-500');
            this.classList.add('border-blue-500', 'text-blue-600');

            document.querySelectorAll('.tab-content').forEach(content => {
                content.classList.remove('active');
            });
            document.getElementById(tab + '-tab').classList.add('active');

            if (tab === 'containers') {
                startContainersAutoRefresh();
            } else {
                stopContainersAutoRefresh();
            }

            if (tab === 'compose') {
                loadComposeProjects();
            }
        });
    });
}

// 初始化
document.addEventListener('DOMContentLoaded', async function() {
    // 初始化国际化
    i18n.init();
    
    // 初始化主题
    ThemeManager.init();
    
    // Compose 编辑器键盘支持
    const editor = DOM.get('compose-editor');
    if (editor) {
        editor.addEventListener('keydown', function(e) {
            if (e.key === 'Tab') {
                e.preventDefault();
                const start = this.selectionStart;
                const end = this.selectionEnd;
                this.value = this.value.substring(0, start) + '  ' + this.value.substring(end);
                this.selectionStart = this.selectionEnd = start + 2;
            }
            if ((e.ctrlKey || e.metaKey) && e.key === 's') {
                e.preventDefault();
                if (currentComposeProject) saveComposeFile();
            }
        });
    }

    // 绑定表单
    DOM.get('login-form').addEventListener('submit', handleLogin);
    DOM.get('change-password-form').addEventListener('submit', handleChangePassword);
    
    // 初始化标签页
    initTabs();

    // 检查登录状态
    const isAuthenticated = await checkAuth();
    if (isAuthenticated) {
        if (needChangePassword) {
            DOM.get('login-page').classList.add('hidden');
            showChangePasswordModal(false);
        } else {
            showMainPage();
        }
    } else {
        DOM.get('login-page').classList.remove('hidden');
        DOM.get('main-page').classList.add('hidden');
    }

    // 模态框点击外部关闭
    DOM.get('log-modal')?.addEventListener('click', function(e) {
        if (e.target === this) closeLogModal();
    });

    DOM.get('change-password-modal')?.addEventListener('click', function(e) {
        if (e.target === this && !needChangePassword) this.classList.remove('active');
    });
});

// 页面卸载清理
window.addEventListener('beforeunload', function() {
    if (logEventSource) logEventSource.close();
    stopAllIntervals();
});
