/**
 * 工具函数模块
 */

// ===== 防抖函数 =====
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// ===== DOM 元素缓存 =====
const DOM = {
    _cache: {},
    get(id) {
        if (!this._cache[id]) {
            this._cache[id] = document.getElementById(id);
        }
        return this._cache[id];
    },
    clear() {
        this._cache = {};
    }
};

// ===== Toast 通知 =====
function showToast(message, type = 'success', options = {}) {
    const container = DOM.get('toast-container');
    const duration = options.duration || (type === 'error' ? 6000 : 3000);
    const title = options.title || (type === 'success' ? '成功' : type === 'error' ? '错误' : '提示');
    
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.style.position = 'relative';
    
    const icons = {
        success: '<svg class="toast-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>',
        error: '<svg class="toast-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>',
        warning: '<svg class="toast-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg>'
    };
    
    toast.innerHTML = `
        ${icons[type] || icons.success}
        <div class="toast-content">
            <div class="toast-title">${title}</div>
            <div class="toast-message">${message}</div>
        </div>
        <button class="toast-close" onclick="this.parentElement.remove()">&times;</button>
        <div class="toast-progress" style="width: 100%; transition: width ${duration}ms linear;"></div>
    `;
    
    container.appendChild(toast);
    
    requestAnimationFrame(() => {
        toast.classList.add('show');
        const progress = toast.querySelector('.toast-progress');
        if (progress) progress.style.width = '0%';
    });
    
    const timer = setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, duration);
    
    toast.addEventListener('mouseenter', () => {
        clearTimeout(timer);
        const progress = toast.querySelector('.toast-progress');
        if (progress) progress.style.transition = 'none';
    });
}

// ===== 确认弹窗 =====
function showConfirm(options) {
    return new Promise((resolve) => {
        const modal = DOM.get('confirm-modal');
        const titleEl = DOM.get('confirm-title');
        const messageEl = DOM.get('confirm-message');
        const iconEl = DOM.get('confirm-icon');
        const okBtn = DOM.get('confirm-ok');
        const cancelBtn = DOM.get('confirm-cancel');
        
        titleEl.textContent = options.title || '确认操作';
        messageEl.innerHTML = options.message || '确定要执行此操作吗？';
        iconEl.className = 'confirm-icon ' + (options.type || 'danger');
        okBtn.className = 'confirm-btn ' + (options.type === 'danger' ? 'confirm-btn-danger' : 'confirm-btn-primary');
        okBtn.textContent = options.confirmText || '确认';
        cancelBtn.textContent = options.cancelText || '取消';
        
        modal.classList.add('active');
        
        const cleanup = () => {
            modal.classList.remove('active');
            okBtn.onclick = null;
            cancelBtn.onclick = null;
        };
        
        okBtn.onclick = () => { cleanup(); resolve(true); };
        cancelBtn.onclick = () => { cleanup(); resolve(false); };
        modal.onclick = (e) => {
            if (e.target === modal) { cleanup(); resolve(false); }
        };
    });
}

// ===== 深色模式 =====
const ThemeManager = {
    init() {
        const saved = localStorage.getItem('theme');
        if (saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
            document.documentElement.classList.add('dark');
        }
        this.updateIcon();
    },
    toggle() {
        document.documentElement.classList.toggle('dark');
        const isDark = document.documentElement.classList.contains('dark');
        localStorage.setItem('theme', isDark ? 'dark' : 'light');
        this.updateIcon();
    },
    updateIcon() {
        const btn = DOM.get('theme-toggle');
        if (!btn) return;
        const isDark = document.documentElement.classList.contains('dark');
        btn.innerHTML = isDark 
            ? '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"></path></svg>'
            : '<svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path></svg>';
    }
};

// ===== 分页管理器 =====
class Paginator {
    constructor(options = {}) {
        this.pageSize = options.pageSize || 10;
        this.currentPage = 1;
        this.data = [];
        this.filteredData = [];
        this.onRender = options.onRender || (() => {});
        this.containerId = options.containerId;
    }
    
    setData(data) {
        this.data = data;
        this.filteredData = data;
        this.currentPage = 1;
    }
    
    filter(filterFn) {
        this.filteredData = this.data.filter(filterFn);
        this.currentPage = 1;
        this.render();
    }
    
    sort(key, direction = 'asc') {
        this.filteredData.sort((a, b) => {
            let valA = a[key], valB = b[key];
            if (typeof valA === 'string') valA = valA.toLowerCase();
            if (typeof valB === 'string') valB = valB.toLowerCase();
            if (valA < valB) return direction === 'asc' ? -1 : 1;
            if (valA > valB) return direction === 'asc' ? 1 : -1;
            return 0;
        });
        this.render();
    }
    
    getPageData() {
        const start = (this.currentPage - 1) * this.pageSize;
        return this.filteredData.slice(start, start + this.pageSize);
    }
    
    getTotalPages() {
        return Math.ceil(this.filteredData.length / this.pageSize);
    }
    
    goToPage(page) {
        const total = this.getTotalPages();
        if (page < 1) page = 1;
        if (page > total) page = total;
        this.currentPage = page;
        this.render();
    }
    
    render() {
        this.onRender(this.getPageData());
        this.renderPagination();
    }
    
    renderPagination() {
        const container = DOM.get(this.containerId);
        if (!container) return;
        
        const total = this.getTotalPages();
        const current = this.currentPage;
        
        if (total <= 1) {
            container.innerHTML = '';
            return;
        }
        
        let html = `<div class="pagination">
            <span class="pagination-info">共 ${this.filteredData.length} 条</span>
            <button ${current === 1 ? 'disabled' : ''} onclick="window.paginators['${this.containerId}'].goToPage(1)">首页</button>
            <button ${current === 1 ? 'disabled' : ''} onclick="window.paginators['${this.containerId}'].goToPage(${current - 1})">上一页</button>`;
        
        // 页码按钮
        const range = 2;
        for (let i = Math.max(1, current - range); i <= Math.min(total, current + range); i++) {
            html += `<button class="${i === current ? 'active' : ''}" onclick="window.paginators['${this.containerId}'].goToPage(${i})">${i}</button>`;
        }
        
        html += `<button ${current === total ? 'disabled' : ''} onclick="window.paginators['${this.containerId}'].goToPage(${current + 1})">下一页</button>
            <button ${current === total ? 'disabled' : ''} onclick="window.paginators['${this.containerId}'].goToPage(${total})">末页</button>
            <select class="page-size-select" onchange="window.paginators['${this.containerId}'].setPageSize(this.value)">
                <option value="10" ${this.pageSize === 10 ? 'selected' : ''}>10条/页</option>
                <option value="20" ${this.pageSize === 20 ? 'selected' : ''}>20条/页</option>
                <option value="50" ${this.pageSize === 50 ? 'selected' : ''}>50条/页</option>
            </select>
        </div>`;
        
        container.innerHTML = html;
    }
    
    setPageSize(size) {
        this.pageSize = parseInt(size);
        this.currentPage = 1;
        this.render();
    }
}

// 全局分页器存储
window.paginators = {};
