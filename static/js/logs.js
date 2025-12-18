/**
 * 日志模块
 */

let logEventSource = null;
let currentLogContent = '';
let currentLogContainerName = '';

// 查看容器日志
function viewLogs(id, name) {
    const modal = DOM.get('log-modal');
    const logContainer = DOM.get('log-container');
    const titleEl = DOM.get('log-modal-title');
    const searchInput = DOM.get('log-search');
    
    modal.classList.add('active');
    logContainer.innerHTML = '正在加载日志...\n';
    titleEl.textContent = `容器日志 - ${name}`;
    currentLogContainerName = name;
    currentLogContent = '';
    searchInput.value = '';

    if (logEventSource) {
        logEventSource.close();
        logEventSource = null;
    }

    const eventSource = new EventSource(`/api/containers/logs?id=${id}`);
    logEventSource = eventSource;

    eventSource.onmessage = function(event) {
        if (event.data) {
            currentLogContent += event.data + '\n';
            const searchText = searchInput.value.toLowerCase();
            if (searchText) {
                filterLogs();
            } else {
                logContainer.textContent = currentLogContent;
            }
            if (DOM.get('auto-scroll').checked) {
                logContainer.scrollTop = logContainer.scrollHeight;
            }
        }
    };

    eventSource.onerror = function(error) {
        console.error('日志流错误:', error);
        if (eventSource.readyState === EventSource.CLOSED) {
            if (currentLogContent === '') {
                logContainer.textContent = '获取日志失败，请检查容器状态或容器是否正在运行';
            } else {
                currentLogContent += '\n[日志流已断开]';
                logContainer.textContent = currentLogContent;
            }
            eventSource.close();
            logEventSource = null;
        }
    };
}

// 筛选日志（带高亮）
const filterLogs = debounce(function() {
    const searchText = DOM.get('log-search').value.toLowerCase();
    const logContainer = DOM.get('log-container');
    
    if (!searchText) {
        logContainer.textContent = currentLogContent;
        return;
    }
    
    const lines = currentLogContent.split('\n');
    const filtered = lines.filter(line => line.toLowerCase().includes(searchText));
    
    if (filtered.length === 0) {
        logContainer.innerHTML = '<span class="text-gray-500">没有匹配的日志</span>';
        return;
    }
    
    // 高亮匹配文本
    const highlighted = filtered.map(line => {
        const regex = new RegExp(`(${escapeRegex(searchText)})`, 'gi');
        return line.replace(regex, '<span class="log-highlight">$1</span>');
    }).join('\n');
    
    logContainer.innerHTML = highlighted;
}, 300);

// 转义正则特殊字符
function escapeRegex(string) {
    return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// 下载日志
function downloadLogs() {
    if (!currentLogContent) {
        showToast('没有可下载的日志', 'warning');
        return;
    }
    
    const blob = new Blob([currentLogContent], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${currentLogContainerName || 'container'}-logs-${new Date().toISOString().slice(0,19).replace(/:/g,'-')}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    showToast('日志已下载', 'success');
}

// 关闭日志弹窗
function closeLogModal() {
    DOM.get('log-modal').classList.remove('active');
    if (logEventSource) {
        logEventSource.close();
        logEventSource = null;
    }
    DOM.get('log-container').textContent = '';
}
