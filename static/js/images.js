/**
 * 镜像管理模块
 */

let allImagesData = [];
let imageSortKey = 'name';
let imageSortDir = 'asc';

// 镜像分页器
const imagePaginator = new Paginator({
    pageSize: 10,
    containerId: 'images-pagination',
    onRender: renderImagesTable
});
window.paginators['images-pagination'] = imagePaginator;

// 加载镜像列表
async function loadImages() {
    try {
        const response = await authFetch('/api/images');
        if (!response.ok) throw new Error(await response.text() || '获取镜像列表失败');
        
        const data = await response.json();
        if (!Array.isArray(data)) {
            console.error('返回的数据不是数组:', data);
            return;
        }
        
        allImagesData = data;
        imagePaginator.setData(data);
        applyImageSort();
        filterImages();
    } catch (error) {
        console.error('加载镜像列表失败:', error);
        showToast(error.message, 'error', { title: '加载镜像列表失败' });
    }
}

// 筛选镜像（带防抖）
const filterImages = debounce(function() {
    const searchText = DOM.get('image-search').value.toLowerCase();
    
    imagePaginator.filter(image => {
        return !searchText || 
            image.name.toLowerCase().includes(searchText) || 
            image.tag.toLowerCase().includes(searchText) ||
            image.id.toLowerCase().includes(searchText);
    });
}, 300);

// 排序镜像
function sortImages(key) {
    if (imageSortKey === key) {
        imageSortDir = imageSortDir === 'asc' ? 'desc' : 'asc';
    } else {
        imageSortKey = key;
        imageSortDir = 'asc';
    }
    applyImageSort();
    updateSortIcons('image');
}

function applyImageSort() {
    imagePaginator.sort(imageSortKey, imageSortDir);
}

// 渲染镜像表格
function renderImagesTable(data) {
    const tbody = DOM.get('images-tbody');
    
    if (!data || data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6" class="px-4 py-8 text-center text-gray-500 dark:text-dark-muted">${t('image.empty')}</td></tr>`;
        return;
    }

    tbody.innerHTML = data.map(image => `
        <tr class="hover:bg-gray-50 dark:hover:bg-dark-border transition-colors">
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${image.id}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${image.name}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${image.tag}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${image.size}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${image.created}</td>
            <td class="px-4 py-3 text-sm">
                <button onclick="removeImage('${image.id}', '${image.name}:${image.tag}')" class="action-btn bg-red-500 text-white rounded text-xs hover:bg-red-600">${t('image.remove')}</button>
            </td>
        </tr>
    `).join('');
}

// 删除镜像
async function removeImage(id, name) {
    const confirmed = await showConfirm({
        title: '删除镜像',
        message: `确定要删除镜像 <strong>${name}</strong> 吗？<br><span style="color:#6b7280;font-size:12px;">如果有容器正在使用此镜像，删除将会失败</span>`,
        type: 'danger',
        confirmText: '确认删除'
    });
    if (!confirmed) return;

    try {
        const response = await authFetch('/api/images/remove', {
            method: 'POST',
            body: JSON.stringify({ id })
        });

        if (!response.ok) throw new Error(await response.text());
        showToast(`镜像 ${name} 已删除`, 'success', { title: '删除成功' });
        loadImages();
    } catch (error) {
        showToast(error.message, 'error', { title: '删除失败' });
        loadImages();
    }
}

// 刷新镜像
async function refreshImages() {
    const icon = DOM.get('refresh-images-icon');
    icon.classList.add('refresh-spinning');
    await loadImages();
    setTimeout(() => icon.classList.remove('refresh-spinning'), 300);
}

// ========== 构建镜像功能 ==========

// 打开构建镜像模态框
function openBuildImageModal() {
    document.getElementById('build-image-name').value = '';
    document.getElementById('build-image-tag').value = 'latest';
    document.getElementById('build-dockerfile').value = `FROM alpine:latest

# 安装依赖
RUN apk add --no-cache bash

# 设置工作目录
WORKDIR /app

# 复制文件
# COPY . .

# 启动命令
CMD ["sh"]`;
    document.getElementById('build-output').innerHTML = '';
    document.getElementById('build-output-container').classList.add('hidden');
    document.getElementById('build-image-modal').classList.add('active');
}

// 关闭构建镜像模态框
function closeBuildImageModal() {
    document.getElementById('build-image-modal').classList.remove('active');
}

// 上传 Dockerfile 文件
function handleDockerfileUpload(input) {
    const file = input.files[0];
    if (!file) return;

    // 限制文件大小 1MB
    if (file.size > 1024 * 1024) {
        showToast(t('build.fileTooLarge'), 'error');
        input.value = '';
        return;
    }

    const reader = new FileReader();
    reader.onload = function(e) {
        const content = e.target.result;
        document.getElementById('build-dockerfile').value = content;
        showToast(t('build.fileLoaded'), 'success');
    };
    reader.onerror = function() {
        showToast(t('build.fileReadError'), 'error');
    };
    reader.readAsText(file);
    
    // 清空 input，允许重复上传同一文件
    input.value = '';
}

// 构建镜像
async function buildImage() {
    const imageName = document.getElementById('build-image-name').value.trim();
    const tag = document.getElementById('build-image-tag').value.trim() || 'latest';
    const dockerfile = document.getElementById('build-dockerfile').value;

    if (!imageName) {
        showToast(t('build.nameRequired'), 'error');
        return;
    }

    if (!dockerfile.trim()) {
        showToast(t('build.dockerfileRequired'), 'error');
        return;
    }

    // 显示输出区域
    const outputContainer = document.getElementById('build-output-container');
    const output = document.getElementById('build-output');
    outputContainer.classList.remove('hidden');
    output.innerHTML = '<div class="text-blue-400">' + t('build.starting') + '</div>';

    // 禁用按钮
    const btn = document.getElementById('build-btn');
    const originalText = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<span class="inline-flex items-center"><svg class="animate-spin -ml-1 mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>' + t('build.building') + '</span>';

    try {
        const response = await authFetch('/api/images/build', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                image_name: imageName,
                tag: tag,
                dockerfile: dockerfile
            })
        });

        // 读取 SSE 流
        const reader = response.body.getReader();
        const decoder = new TextDecoder();

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const text = decoder.decode(value);
            const lines = text.split('\n');

            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    try {
                        const data = JSON.parse(line.slice(6));
                        if (data.type === 'log') {
                            output.innerHTML += '<div class="text-gray-300">' + escapeHtml(data.message) + '</div>';
                        } else if (data.type === 'error') {
                            output.innerHTML += '<div class="text-red-400">❌ ' + escapeHtml(data.message) + '</div>';
                        } else if (data.type === 'success') {
                            output.innerHTML += '<div class="text-green-400">✅ ' + escapeHtml(data.message) + '</div>';
                            showToast(t('build.success'), 'success');
                            loadImages();
                        } else if (data.type === 'start') {
                            output.innerHTML += '<div class="text-blue-400">🚀 ' + escapeHtml(data.message) + '</div>';
                        }
                        output.scrollTop = output.scrollHeight;
                    } catch (e) {
                        // 忽略解析错误
                    }
                }
            }
        }
    } catch (error) {
        output.innerHTML += '<div class="text-red-400">❌ ' + t('build.failed') + ': ' + escapeHtml(error.message) + '</div>';
        showToast(t('build.failed') + ': ' + error.message, 'error');
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalText;
    }
}

// HTML 转义
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
