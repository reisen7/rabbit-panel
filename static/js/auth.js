/**
 * 认证模块
 */

let authToken = '';
let needChangePassword = false;

// 检查登录状态
async function checkAuth() {
    try {
        const response = await fetch('/api/auth/me', { credentials: 'include' });
        if (response.ok) {
            const data = await response.json();
            authToken = document.cookie.match(/token=([^;]+)/)?.[1] || '';
            needChangePassword = data.need_change_password;
            DOM.get('current-user').textContent = `用户: ${data.username}`;
            return true;
        }
        return false;
    } catch (error) {
        return false;
    }
}

// 登录
async function handleLogin(event) {
    event.preventDefault();
    const username = DOM.get('login-username').value;
    const password = DOM.get('login-password').value;
    const errorDiv = DOM.get('login-error');

    try {
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ username, password })
        });

        const data = await response.json();

        if (response.ok) {
            authToken = data.token;
            needChangePassword = data.need_change_password;
            
            if (needChangePassword) {
                DOM.get('login-page').classList.add('hidden');
                showChangePasswordModal(false);
            } else {
                showMainPage();
            }
        } else {
            errorDiv.textContent = data.error || '登录失败';
            errorDiv.classList.remove('hidden');
        }
    } catch (error) {
        errorDiv.textContent = '登录失败: ' + error.message;
        errorDiv.classList.remove('hidden');
    }
}

// 修改密码
async function handleChangePassword(event) {
    event.preventDefault();
    const oldPasswordDiv = DOM.get('old-password-div');
    const oldPasswordInput = DOM.get('old-password');
    const newPassword = DOM.get('new-password').value;
    const confirmPassword = DOM.get('confirm-password').value;
    const errorDiv = DOM.get('change-password-error');

    if (!newPassword) {
        errorDiv.textContent = '新密码不能为空';
        errorDiv.classList.remove('hidden');
        return;
    }

    if (newPassword !== confirmPassword) {
        errorDiv.textContent = '两次输入的密码不一致';
        errorDiv.classList.remove('hidden');
        return;
    }

    if (!oldPasswordDiv.classList.contains('hidden') && !oldPasswordInput.value) {
        errorDiv.textContent = '旧密码不能为空';
        errorDiv.classList.remove('hidden');
        return;
    }

    const oldPassword = oldPasswordDiv.classList.contains('hidden') ? '' : oldPasswordInput.value;

    try {
        const response = await fetch('/api/auth/change-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include',
            body: JSON.stringify({ old_password: needChangePassword ? '' : oldPassword, new_password: newPassword })
        });

        const data = await response.json();

        if (response.ok) {
            authToken = data.token;
            needChangePassword = false;
            DOM.get('change-password-modal').classList.remove('active');
            showMainPage();
            showToast('密码修改成功', 'success');
        } else {
            errorDiv.textContent = data.error || '修改密码失败';
            errorDiv.classList.remove('hidden');
        }
    } catch (error) {
        errorDiv.textContent = '修改密码失败: ' + error.message;
        errorDiv.classList.remove('hidden');
    }
}

// 登出
async function handleLogout() {
    try {
        await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' });
    } catch (error) {
        console.error('登出失败:', error);
    }
    authToken = '';
    DOM.get('main-page').classList.add('hidden');
    DOM.get('login-page').classList.remove('hidden');
    stopAllIntervals();
}

// 显示修改密码模态框
function showChangePasswordModal(showOldPassword = false) {
    const modal = DOM.get('change-password-modal');
    const oldPasswordDiv = DOM.get('old-password-div');
    const errorDiv = DOM.get('change-password-error');
    const form = DOM.get('change-password-form');
    
    errorDiv.classList.add('hidden');
    errorDiv.textContent = '';
    form.reset();
    
    if (showOldPassword) {
        oldPasswordDiv.classList.remove('hidden');
    } else {
        oldPasswordDiv.classList.add('hidden');
    }
    
    modal.classList.add('active');
}

// 带认证的 fetch 请求
async function authFetch(url, options = {}) {
    const defaultOptions = {
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', ...options.headers }
    };

    const response = await fetch(url, { ...defaultOptions, ...options });
    
    if (response.status === 401) {
        handleLogout();
        throw new Error('未授权，请重新登录');
    }

    if (response.status === 403) {
        const data = await response.json();
        if (data.need_change_password) {
            DOM.get('main-page').classList.add('hidden');
            showChangePasswordModal(true);
        }
        throw new Error(data.error || '禁止访问');
    }

    return response;
}
