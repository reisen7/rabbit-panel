/**
 * 国际化模块
 */

const i18n = {
    currentLang: 'zh',
    
    // 语言包
    messages: {
        zh: {
            // 通用
            'app.title': '容器运维面板',
            'common.loading': '加载中...',
            'common.save': '保存',
            'common.cancel': '取消',
            'common.confirm': '确认',
            'common.delete': '删除',
            'common.refresh': '刷新',
            'common.search': '搜索',
            'common.create': '创建',
            'common.edit': '编辑',
            'common.close': '关闭',
            'common.detail': '详情',
            'common.actions': '操作',
            'common.created': '创建时间',
            'common.success': '成功',
            'common.error': '错误',
            'common.warning': '警告',
            
            // 登录
            'login.title': '容器运维面板',
            'login.username': '用户名',
            'login.password': '密码',
            'login.submit': '登录',
            'login.logout': '登出',
            
            // 修改密码
            'password.title': '修改密码',
            'password.hint': '首次登录需要修改密码，密码必须包含：大写字母、小写字母、数字、特殊字符，长度至少8位',
            'password.old': '旧密码',
            'password.new': '新密码',
            'password.confirm': '确认新密码',
            'password.submit': '确认修改',
            
            // 系统监控
            'monitor.cpu': 'CPU 使用率',
            'monitor.memory': '内存使用率',
            'monitor.disk': '磁盘使用率',
            'monitor.time': '当前时间',
            
            // 标签页
            'tab.containers': '容器管理',
            'tab.images': '镜像管理',
            'tab.networks': '网络管理',
            'tab.compose': 'Compose 管理',
            
            // 容器管理
            'container.list': '容器列表',
            'container.search': '搜索名称/镜像...',
            'container.status.all': '全部状态',
            'container.status.running': '运行中',
            'container.status.stopped': '已停止',
            'container.create': '创建容器',
            'container.id': 'ID',
            'container.name': '名称',
            'container.image': '镜像',
            'container.status': '状态',
            'container.ports': '端口',
            'container.resources': '资源',
            'container.filesystem': '文件系统',
            'container.created': '创建时间',
            'container.actions': '操作',
            'container.start': '启动',
            'container.stop': '停止',
            'container.restart': '重启',
            'container.logs': '日志',
            'container.terminal': '终端',
            'container.files': '文件',
            'container.config': '配置',
            'container.remove': '删除',
            'container.empty': '暂无匹配的容器',
            
            // 创建容器
            'create.title': '创建容器',
            'create.image': '镜像名称',
            'create.image.placeholder': 'nginx:latest',
            'create.name': '容器名称',
            'create.name.placeholder': 'my-container (可选)',
            'create.ports': '端口映射',
            'create.port.host': '主机端口',
            'create.port.container': '容器端口',
            'create.envs': '环境变量',
            'create.env.key': '变量名',
            'create.env.value': '值',
            'create.volumes': '数据卷挂载',
            'create.vol.host': '主机路径',
            'create.vol.container': '容器路径',
            'create.restart': '重启策略',
            'create.restart.no': '不自动重启',
            'create.restart.always': '总是重启',
            'create.restart.unless': '除非手动停止',
            'create.restart.failure': '失败时重启',
            'create.network': '网络模式',
            'create.network.default': '默认 (bridge)',
            'create.preview': '命令预览',
            'create.submit': '创建并运行',
            
            // 镜像管理
            'image.list': '镜像列表',
            'image.search': '搜索名称/标签...',
            'image.id': 'ID',
            'image.name': '名称',
            'image.tag': '标签',
            'image.size': '大小',
            'image.created': '创建时间',
            'image.actions': '操作',
            'image.remove': '删除',
            'image.empty': '暂无匹配的镜像',
            
            // 构建镜像
            'build.title': '构建镜像',
            'build.imageName': '镜像名称',
            'build.tag': '标签',
            'build.hint': '提示：仅支持基于基础镜像构建，不支持 COPY 本地文件',
            'build.output': '构建输出',
            'build.start': '开始构建',
            'build.building': '构建中...',
            'build.starting': '正在准备构建环境...',
            'build.success': '镜像构建成功',
            'build.failed': '构建失败',
            'build.nameRequired': '镜像名称不能为空',
            'build.dockerfileRequired': 'Dockerfile 内容不能为空',
            'build.upload': '上传文件',
            'build.fileLoaded': 'Dockerfile 已加载',
            'build.fileTooLarge': '文件过大，最大支持 1MB',
            'build.fileReadError': '读取文件失败',
            
            // Compose 管理
            'compose.projects': 'Compose 项目',
            'compose.list': '项目列表',
            'compose.new': '新建项目',
            'compose.name': '项目名称 (英文)',
            'compose.up': '启动',
            'compose.down': '停止',
            'compose.pull': '拉取',
            'compose.status': '容器状态',
            'compose.select': '选择项目查看详情',
            'compose.upload': '上传',
            'compose.fileLoaded': '文件已加载',
            'compose.fileTooLarge': '文件过大，最大支持 1MB',
            'compose.fileReadError': '读取文件失败',
            
            // 网络管理
            'network.list': '网络列表',
            'network.name': '名称',
            'network.driver': '驱动',
            'network.scope': '范围',
            'network.containers': '容器数',
            'network.internal': '内部网络',
            'network.create': '创建网络',
            'network.delete': '删除网络',
            'network.detail': '网络详情',
            'network.empty': '暂无网络',
            'network.loadFailed': '加载网络列表失败',
            'network.nameRequired': '网络名称不能为空',
            'network.createSuccess': '网络创建成功',
            'network.createFailed': '创建网络失败',
            'network.deleteConfirm': '确定要删除网络',
            'network.deleteSuccess': '网络已删除',
            'network.deleteFailed': '删除网络失败',
            'network.noContainers': '暂无连接的容器',
            'network.connectedContainers': '连接的容器',
            
            // 日志
            'logs.title': '容器日志',
            'logs.search': '搜索日志...',
            'logs.autoscroll': '自动滚动',
            'logs.download': '下载日志',
            
            // 终端
            'terminal.title': '终端',
            'terminal.placeholder': '输入命令...',
            'terminal.execute': '执行',
            'terminal.connected': '已连接到容器',
            'terminal.hint': '输入命令并按回车执行，支持 ↑↓ 浏览历史命令',
            
            // 文件管理
            'files.title': '文件管理',
            'files.parent': '上级',
            'files.mkdir': '新建目录',
            'files.upload': '上传',
            'files.name': '名称',
            'files.size': '大小',
            'files.mode': '权限',
            'files.modified': '修改时间',
            'files.actions': '操作',
            'files.edit': '编辑文件',
            'files.empty': '目录为空',
            
            // 容器配置
            'config.title': '容器配置',
            'config.image': '镜像',
            'config.state': '状态',
            'config.created': '创建时间',
            'config.name': '容器名称',
            'config.rename': '重命名',
            'config.restart': '重启策略',
            'config.memory': '内存限制 (MB)',
            'config.cpus': 'CPU 限制 (核)',
            'config.save': '保存配置',
            'config.imageAddr': '镜像地址',
            'config.imageHint': '升级时只需修改标签即可',
            'config.imageRequired': '镜像地址不能为空',
            
            // 通用操作结果
            'common.saveSuccess': '保存成功',
            'common.saveFailed': '保存失败',
            'common.deleteSuccess': '删除成功',
            'common.deleteFailed': '删除失败',
            
            // 容器状态
            'containers.running': '运行中',
            'containers.stopped': '已停止',
            
            // 终端
            'terminal.exitCode': '退出码',
            'terminal.execFailed': '执行失败',
            
            // 文件管理
            'files.enterDirName': '请输入目录名称',
            'files.createSuccess': '目录创建成功',
            'files.createFailed': '创建失败',
            'files.sizeLimit': '文件大小不能超过 10MB',
            'files.uploadSuccess': '上传成功',
            'files.uploadFailed': '上传失败',
            'files.downloadFailed': '下载失败',
            'files.readFailed': '读取文件失败',
            'files.directory': '目录',
            'files.file': '文件',
            'files.confirmDelete': '确定要删除这个',
            'files.download': '下载',
            
            // 容器配置
            'config.noPorts': '无端口映射',
            'config.noLabels': '无标签',
            'config.unlimited': '不限制',
            'config.cores': '核',
            'config.enabled': '启用',
            'config.disabled': '禁用',
            'config.getFailed': '获取配置失败',
            'config.updateSuccess': '配置已更新',
            'config.updateFailed': '更新失败',
            'config.nameRequired': '名称不能为空',
            'config.renameSuccess': '重命名成功',
            'config.renameFailed': '重命名失败',
            'config.noConfig': '没有配置信息',
            'config.recreate': '重建容器',
            'config.recreateConfirm': '确定要重建容器吗？',
            'config.recreateWarning': '容器内未挂载的数据将丢失！',
            'config.confirmRecreate': '确认重建',
            'config.recreating': '正在重建...',
            'config.recreateSuccess': '容器重建成功',
            'config.recreateFailed': '重建失败',
            
            // 确认框
            'confirm.delete.title': '删除容器',
            'confirm.delete.message': '确定要删除容器 <strong>{name}</strong> 吗？',
            'confirm.delete.warning': '此操作不可恢复！',
            'confirm.delete.confirm': '确认删除',
            'confirm.stop.title': '停止容器',
            'confirm.stop.message': '确定要停止容器 <strong>{name}</strong> 吗？',
            'confirm.stop.confirm': '确认停止',
        },
        
        en: {
            // Common
            'app.title': 'Container Panel',
            'common.loading': 'Loading...',
            'common.save': 'Save',
            'common.cancel': 'Cancel',
            'common.confirm': 'Confirm',
            'common.delete': 'Delete',
            'common.refresh': 'Refresh',
            'common.search': 'Search',
            'common.create': 'Create',
            'common.edit': 'Edit',
            'common.close': 'Close',
            'common.detail': 'Detail',
            'common.actions': 'Actions',
            'common.created': 'Created',
            'common.success': 'Success',
            'common.error': 'Error',
            'common.warning': 'Warning',
            
            // Login
            'login.title': 'Container Panel',
            'login.username': 'Username',
            'login.password': 'Password',
            'login.submit': 'Login',
            'login.logout': 'Logout',
            
            // Change Password
            'password.title': 'Change Password',
            'password.hint': 'First login requires password change. Password must contain: uppercase, lowercase, numbers, special characters, at least 8 characters',
            'password.old': 'Old Password',
            'password.new': 'New Password',
            'password.confirm': 'Confirm Password',
            'password.submit': 'Confirm',
            
            // System Monitor
            'monitor.cpu': 'CPU Usage',
            'monitor.memory': 'Memory Usage',
            'monitor.disk': 'Disk Usage',
            'monitor.time': 'Current Time',
            
            // Tabs
            'tab.containers': 'Containers',
            'tab.images': 'Images',
            'tab.networks': 'Networks',
            'tab.compose': 'Compose',
            
            // Container Management
            'container.list': 'Container List',
            'container.search': 'Search name/image...',
            'container.status.all': 'All Status',
            'container.status.running': 'Running',
            'container.status.stopped': 'Stopped',
            'container.create': 'Create',
            'container.id': 'ID',
            'container.name': 'Name',
            'container.image': 'Image',
            'container.status': 'Status',
            'container.ports': 'Ports',
            'container.resources': 'Resources',
            'container.filesystem': 'FS',
            'container.created': 'Created',
            'container.actions': 'Actions',
            'container.start': 'Start',
            'container.stop': 'Stop',
            'container.restart': 'Restart',
            'container.logs': 'Logs',
            'container.terminal': 'Term',
            'container.files': 'Files',
            'container.config': 'Config',
            'container.remove': 'Remove',
            'container.empty': 'No containers found',
            
            // Create Container
            'create.title': 'Create Container',
            'create.image': 'Image Name',
            'create.image.placeholder': 'nginx:latest',
            'create.name': 'Container Name',
            'create.name.placeholder': 'my-container (optional)',
            'create.ports': 'Port Mapping',
            'create.port.host': 'Host Port',
            'create.port.container': 'Container Port',
            'create.envs': 'Environment Variables',
            'create.env.key': 'Key',
            'create.env.value': 'Value',
            'create.volumes': 'Volume Mounts',
            'create.vol.host': 'Host Path',
            'create.vol.container': 'Container Path',
            'create.restart': 'Restart Policy',
            'create.restart.no': 'No',
            'create.restart.always': 'Always',
            'create.restart.unless': 'Unless Stopped',
            'create.restart.failure': 'On Failure',
            'create.network': 'Network Mode',
            'create.network.default': 'Default (bridge)',
            'create.preview': 'Command Preview',
            'create.submit': 'Create & Run',
            
            // Image Management
            'image.list': 'Image List',
            'image.search': 'Search name/tag...',
            'image.id': 'ID',
            'image.name': 'Name',
            'image.tag': 'Tag',
            'image.size': 'Size',
            'image.created': 'Created',
            'image.actions': 'Actions',
            'image.remove': 'Remove',
            'image.empty': 'No images found',
            
            // Build Image
            'build.title': 'Build Image',
            'build.imageName': 'Image Name',
            'build.tag': 'Tag',
            'build.hint': 'Note: Only supports building from base images, COPY local files is not supported',
            'build.output': 'Build Output',
            'build.start': 'Start Build',
            'build.building': 'Building...',
            'build.starting': 'Preparing build environment...',
            'build.success': 'Image built successfully',
            'build.failed': 'Build failed',
            'build.nameRequired': 'Image name is required',
            'build.dockerfileRequired': 'Dockerfile content is required',
            'build.upload': 'Upload File',
            'build.fileLoaded': 'Dockerfile loaded',
            'build.fileTooLarge': 'File too large, max 1MB',
            'build.fileReadError': 'Failed to read file',
            
            // Compose Management
            'compose.projects': 'Compose Projects',
            'compose.list': 'Project List',
            'compose.new': 'New Project',
            'compose.name': 'Project Name',
            'compose.up': 'Up',
            'compose.down': 'Down',
            'compose.pull': 'Pull',
            'compose.status': 'Container Status',
            'compose.select': 'Select a project to view details',
            'compose.upload': 'Upload',
            'compose.fileLoaded': 'File loaded',
            'compose.fileTooLarge': 'File too large, max 1MB',
            'compose.fileReadError': 'Failed to read file',
            
            // Network Management
            'network.list': 'Network List',
            'network.name': 'Name',
            'network.driver': 'Driver',
            'network.scope': 'Scope',
            'network.containers': 'Containers',
            'network.internal': 'Internal',
            'network.create': 'Create Network',
            'network.delete': 'Delete Network',
            'network.detail': 'Network Details',
            'network.empty': 'No networks found',
            'network.loadFailed': 'Failed to load networks',
            'network.nameRequired': 'Network name is required',
            'network.createSuccess': 'Network created',
            'network.createFailed': 'Failed to create network',
            'network.deleteConfirm': 'Are you sure to delete network',
            'network.deleteSuccess': 'Network deleted',
            'network.deleteFailed': 'Failed to delete network',
            'network.noContainers': 'No connected containers',
            'network.connectedContainers': 'Connected Containers',
            
            // Logs
            'logs.title': 'Container Logs',
            'logs.search': 'Search logs...',
            'logs.autoscroll': 'Auto Scroll',
            'logs.download': 'Download',
            
            // Terminal
            'terminal.title': 'Terminal',
            'terminal.placeholder': 'Enter command...',
            'terminal.execute': 'Run',
            'terminal.connected': 'Connected to container',
            'terminal.hint': 'Enter command and press Enter. Use ↑↓ for history',
            
            // File Management
            'files.title': 'File Manager',
            'files.parent': 'Parent',
            'files.mkdir': 'New Folder',
            'files.upload': 'Upload',
            'files.name': 'Name',
            'files.size': 'Size',
            'files.mode': 'Mode',
            'files.modified': 'Modified',
            'files.actions': 'Actions',
            'files.edit': 'Edit File',
            'files.empty': 'Empty directory',
            
            // Container Config
            'config.title': 'Container Config',
            'config.image': 'Image',
            'config.state': 'State',
            'config.created': 'Created',
            'config.name': 'Container Name',
            'config.rename': 'Rename',
            'config.restart': 'Restart Policy',
            'config.memory': 'Memory Limit (MB)',
            'config.cpus': 'CPU Limit (cores)',
            'config.save': 'Save Config',
            'config.imageAddr': 'Image Address',
            'config.imageHint': 'Just change the tag to upgrade',
            'config.imageRequired': 'Image address is required',
            
            // Common operation results
            'common.saveSuccess': 'Saved successfully',
            'common.saveFailed': 'Save failed',
            'common.deleteSuccess': 'Deleted successfully',
            'common.deleteFailed': 'Delete failed',
            
            // Container status
            'containers.running': 'Running',
            'containers.stopped': 'Stopped',
            
            // Terminal
            'terminal.exitCode': 'Exit code',
            'terminal.execFailed': 'Execution failed',
            
            // File management
            'files.enterDirName': 'Enter directory name',
            'files.createSuccess': 'Directory created',
            'files.createFailed': 'Create failed',
            'files.sizeLimit': 'File size cannot exceed 10MB',
            'files.uploadSuccess': 'Upload successful',
            'files.uploadFailed': 'Upload failed',
            'files.downloadFailed': 'Download failed',
            'files.readFailed': 'Failed to read file',
            'files.directory': 'directory',
            'files.file': 'file',
            'files.confirmDelete': 'Are you sure to delete this',
            'files.download': 'Download',
            
            // Container config
            'config.noPorts': 'No port mappings',
            'config.noLabels': 'No labels',
            'config.unlimited': 'Unlimited',
            'config.cores': 'cores',
            'config.enabled': 'Enabled',
            'config.disabled': 'Disabled',
            'config.getFailed': 'Failed to get config',
            'config.updateSuccess': 'Config updated',
            'config.updateFailed': 'Update failed',
            'config.nameRequired': 'Name is required',
            'config.renameSuccess': 'Renamed successfully',
            'config.renameFailed': 'Rename failed',
            'config.noConfig': 'No config info',
            'config.recreate': 'Recreate Container',
            'config.recreateConfirm': 'Are you sure to recreate the container?',
            'config.recreateWarning': 'Unmounted data will be lost!',
            'config.confirmRecreate': 'Confirm Recreate',
            'config.recreating': 'Recreating...',
            'config.recreateSuccess': 'Container recreated successfully',
            'config.recreateFailed': 'Recreate failed',
            
            // Confirm Dialog
            'confirm.delete.title': 'Delete Container',
            'confirm.delete.message': 'Are you sure to delete container <strong>{name}</strong>?',
            'confirm.delete.warning': 'This action cannot be undone!',
            'confirm.delete.confirm': 'Delete',
            'confirm.stop.title': 'Stop Container',
            'confirm.stop.message': 'Are you sure to stop container <strong>{name}</strong>?',
            'confirm.stop.confirm': 'Stop',
        }
    },
    
    // 初始化
    init() {
        // 从 localStorage 读取语言设置
        const savedLang = localStorage.getItem('rabbit-panel-lang');
        if (savedLang && this.messages[savedLang]) {
            this.currentLang = savedLang;
        } else {
            // 根据浏览器语言自动选择
            const browserLang = navigator.language.toLowerCase();
            this.currentLang = browserLang.startsWith('zh') ? 'zh' : 'en';
        }
        this.updateUI();
    },
    
    // 获取翻译
    t(key, params = {}) {
        let text = this.messages[this.currentLang][key] || this.messages['zh'][key] || key;
        // 替换参数
        Object.keys(params).forEach(k => {
            text = text.replace(`{${k}}`, params[k]);
        });
        return text;
    },
    
    // 切换语言
    toggle() {
        this.currentLang = this.currentLang === 'zh' ? 'en' : 'zh';
        localStorage.setItem('rabbit-panel-lang', this.currentLang);
        this.updateUI();
        // 刷新数据显示
        if (typeof loadContainers === 'function') loadContainers(true);
        if (typeof loadImages === 'function') loadImages(true);
    },
    
    // 更新界面文本
    updateUI() {
        // 更新所有带 data-i18n 属性的元素
        document.querySelectorAll('[data-i18n]').forEach(el => {
            const key = el.getAttribute('data-i18n');
            if (el.tagName === 'INPUT' && el.hasAttribute('placeholder')) {
                el.placeholder = this.t(key);
            } else {
                el.textContent = this.t(key);
            }
        });
        
        // 更新语言切换按钮
        const langBtn = document.getElementById('lang-toggle');
        if (langBtn) {
            langBtn.textContent = this.currentLang === 'zh' ? 'EN' : '中';
            langBtn.title = this.currentLang === 'zh' ? 'Switch to English' : '切换到中文';
        }
        
        // 更新页面标题
        document.title = this.t('app.title');
    }
};

// 快捷函数
function t(key, params) {
    return i18n.t(key, params);
}
