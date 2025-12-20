/**
 * 网络管理模块
 */

let allNetworksData = [];
let networkSortKey = 'name';
let networkSortDir = 'asc';

// 网络分页器
const networkPaginator = new Paginator({
    pageSize: 10,
    containerId: 'networks-pagination',
    onRender: renderNetworksTable
});
window.paginators['networks-pagination'] = networkPaginator;

// 加载网络列表
async function loadNetworks() {
    try {
        const response = await authFetch('/api/networks');
        if (!response.ok) throw new Error(await response.text() || '获取网络列表失败');
        
        const data = await response.json();
        if (!Array.isArray(data)) {
            console.error('返回的数据不是数组:', data);
            return;
        }
        
        allNetworksData = data;
        networkPaginator.setData(data);
        applyNetworkSort();
        filterNetworks();
    } catch (error) {
        console.error('加载网络列表失败:', error);
        showToast(error.message, 'error', { title: t('network.loadFailed') });
    }
}

// 筛选网络
const filterNetworks = debounce(function() {
    const searchText = document.getElementById('network-search')?.value.toLowerCase() || '';
    
    networkPaginator.filter(network => {
        return !searchText || 
            network.name.toLowerCase().includes(searchText) || 
            network.driver.toLowerCase().includes(searchText) ||
            network.id.toLowerCase().includes(searchText);
    });
}, 300);

// 排序网络
function sortNetworks(key) {
    if (networkSortKey === key) {
        networkSortDir = networkSortDir === 'asc' ? 'desc' : 'asc';
    } else {
        networkSortKey = key;
        networkSortDir = 'asc';
    }
    applyNetworkSort();
}

function applyNetworkSort() {
    networkPaginator.sort(networkSortKey, networkSortDir);
}

// 渲染网络表格
function renderNetworksTable(data) {
    const tbody = document.getElementById('networks-tbody');
    if (!tbody) return;
    
    if (!data || data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" class="px-4 py-8 text-center text-gray-500 dark:text-dark-muted">${t('network.empty')}</td></tr>`;
        return;
    }

    tbody.innerHTML = data.map(network => {
        const isSystem = ['bridge', 'host', 'none'].includes(network.name);
        return `
        <tr class="hover:bg-gray-50 dark:hover:bg-dark-border transition-colors">
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text hidden md:table-cell">${network.id}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text font-medium">${network.name}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${network.driver}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text hidden sm:table-cell">${network.scope}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text hidden lg:table-cell">${network.ipam}</td>
            <td class="px-4 py-3 text-sm text-gray-900 dark:text-dark-text">${network.containers}</td>
            <td class="px-4 py-3 text-sm">
                <div class="flex gap-1">
                    <button onclick="viewNetworkDetail('${network.id}')" class="action-btn bg-blue-500 text-white rounded text-xs hover:bg-blue-600 whitespace-nowrap">${t('common.detail')}</button>
                    ${!isSystem ? `<button onclick="removeNetwork('${network.id}', '${network.name}')" class="action-btn bg-red-500 text-white rounded text-xs hover:bg-red-600 whitespace-nowrap">${t('common.delete')}</button>` : ''}
                </div>
            </td>
        </tr>
    `}).join('');
}

// 删除网络
async function removeNetwork(id, name) {
    const confirmed = await showConfirm({
        title: t('network.delete'),
        message: `${t('network.deleteConfirm')} <strong>${name}</strong>?`,
        type: 'danger',
        confirmText: t('common.confirm')
    });
    if (!confirmed) return;

    try {
        const response = await authFetch('/api/networks/remove', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id })
        });

        if (!response.ok) throw new Error(await response.text());
        showToast(t('network.deleteSuccess'), 'success');
        loadNetworks();
    } catch (error) {
        showToast(error.message, 'error', { title: t('network.deleteFailed') });
    }
}

// 刷新网络
async function refreshNetworks() {
    const icon = document.getElementById('refresh-networks-icon');
    if (icon) icon.classList.add('refresh-spinning');
    await loadNetworks();
    setTimeout(() => icon?.classList.remove('refresh-spinning'), 300);
}

// 打开创建网络模态框
function openCreateNetworkModal() {
    document.getElementById('create-network-name').value = '';
    document.getElementById('create-network-driver').value = 'bridge';
    document.getElementById('create-network-subnet').value = '';
    document.getElementById('create-network-gateway').value = '';
    document.getElementById('create-network-internal').checked = false;
    document.getElementById('create-network-modal').classList.add('active');
}

// 关闭创建网络模态框
function closeCreateNetworkModal() {
    document.getElementById('create-network-modal').classList.remove('active');
}

// 创建网络
async function createNetwork() {
    const name = document.getElementById('create-network-name').value.trim();
    const driver = document.getElementById('create-network-driver').value;
    const subnet = document.getElementById('create-network-subnet').value.trim();
    const gateway = document.getElementById('create-network-gateway').value.trim();
    const internal = document.getElementById('create-network-internal').checked;

    if (!name) {
        showToast(t('network.nameRequired'), 'error');
        return;
    }

    const btn = document.getElementById('create-network-btn');
    const originalText = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = t('common.loading');

    try {
        const response = await authFetch('/api/networks/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, driver, subnet, gateway, internal })
        });

        if (!response.ok) throw new Error(await response.text());
        
        showToast(t('network.createSuccess'), 'success');
        closeCreateNetworkModal();
        loadNetworks();
    } catch (error) {
        showToast(error.message, 'error', { title: t('network.createFailed') });
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalText;
    }
}

// 查看网络详情
async function viewNetworkDetail(id) {
    try {
        const response = await authFetch('/api/networks/inspect?id=' + id);
        if (!response.ok) throw new Error(await response.text());
        
        const network = await response.json();
        
        // 填充详情
        document.getElementById('network-detail-id').textContent = network.id;
        document.getElementById('network-detail-name').textContent = network.name;
        document.getElementById('network-detail-driver').textContent = network.driver;
        document.getElementById('network-detail-scope').textContent = network.scope;
        document.getElementById('network-detail-internal').textContent = network.internal ? '是' : '否';
        document.getElementById('network-detail-created').textContent = network.created;
        
        // IPAM 配置
        let ipamHtml = '-';
        if (network.ipam && network.ipam.Config && network.ipam.Config.length > 0) {
            ipamHtml = network.ipam.Config.map(c => 
                `<div class="text-sm">Subnet: ${c.Subnet || '-'}, Gateway: ${c.Gateway || '-'}</div>`
            ).join('');
        }
        document.getElementById('network-detail-ipam').innerHTML = ipamHtml;
        
        // 连接的容器
        const containersList = document.getElementById('network-detail-containers');
        if (network.containers && network.containers.length > 0) {
            containersList.innerHTML = network.containers.map(c => `
                <div class="flex justify-between items-center p-2 bg-gray-50 dark:bg-dark-border rounded text-sm">
                    <div>
                        <span class="font-medium">${c.name}</span>
                        <span class="text-gray-500 ml-2">${c.id}</span>
                    </div>
                    <div class="text-gray-600 dark:text-gray-400">${c.ipv4 || '-'}</div>
                </div>
            `).join('');
        } else {
            containersList.innerHTML = '<div class="text-sm text-gray-500">' + t('network.noContainers') + '</div>';
        }
        
        document.getElementById('network-detail-modal').classList.add('active');
    } catch (error) {
        showToast(error.message, 'error');
    }
}

// 关闭网络详情模态框
function closeNetworkDetailModal() {
    document.getElementById('network-detail-modal').classList.remove('active');
}
