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
        setTimeout(loadImages, 500);
    } catch (error) {
        showToast(error.message, 'error', { title: '删除失败' });
    }
}

// 刷新镜像
async function refreshImages() {
    const icon = DOM.get('refresh-images-icon');
    icon.classList.add('refresh-spinning');
    await loadImages();
    setTimeout(() => icon.classList.remove('refresh-spinning'), 300);
}
