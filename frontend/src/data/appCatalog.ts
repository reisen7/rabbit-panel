import { buildImageRef, type PullSource } from '@/utils/imageRef'
import type { ContainerCreateForm } from '@/stores/containerCreate'

export const MIRROR_DOMAIN = 'docker.1ms.run'

export type AppCategory = 'database' | 'mq' | 'web' | 'devops' | 'app'

export interface CatalogApp {
  id: string
  name: string
  category: AppCategory
  repository_name: string
  registry: 'docker.io'
  tag: string
  desc: { zh: string; en: string }
  ports: { host: string; container: string }[]
  volumes: { host: string; container: string }[]
  envs: { key: string; value: string }[]
  note?: { zh: string; en: string }
}

const PASSWORD_TOKEN = '__RANDOM_PASSWORD__'

export const CATALOG_APPS: CatalogApp[] = [
  {
    id: 'mysql',
    name: 'MySQL',
    category: 'database',
    repository_name: 'library/mysql',
    registry: 'docker.io',
    tag: '8',
    desc: { zh: '最流行的开源关系型数据库', en: 'The most popular open-source relational database' },
    ports: [{ host: '3306', container: '3306' }],
    volumes: [{ host: 'mysql-data', container: '/var/lib/mysql' }],
    envs: [
      { key: 'MYSQL_ROOT_PASSWORD', value: PASSWORD_TOKEN },
      { key: 'TZ', value: 'Asia/Shanghai' },
    ],
    note: {
      zh: 'root 密码已随机生成，请在创建前复制保存',
      en: 'A random root password is generated — copy it before creating',
    },
  },
  {
    id: 'mariadb',
    name: 'MariaDB',
    category: 'database',
    repository_name: 'library/mariadb',
    registry: 'docker.io',
    tag: '11',
    desc: { zh: 'MySQL 兼容的开源关系型数据库', en: 'MySQL-compatible open-source relational database' },
    ports: [{ host: '3307', container: '3306' }],
    volumes: [{ host: 'mariadb-data', container: '/var/lib/mysql' }],
    envs: [
      { key: 'MARIADB_ROOT_PASSWORD', value: PASSWORD_TOKEN },
      { key: 'TZ', value: 'Asia/Shanghai' },
    ],
    note: {
      zh: '宿主机使用 3307 端口避免与 MySQL 冲突；root 密码已随机生成，请在创建前复制保存',
      en: 'Host port 3307 avoids clashing with MySQL; a random root password is generated — copy it before creating',
    },
  },
  {
    id: 'postgres',
    name: 'PostgreSQL',
    category: 'database',
    repository_name: 'library/postgres',
    registry: 'docker.io',
    tag: '16',
    desc: { zh: '功能强大的企业级开源关系型数据库', en: 'Powerful enterprise-grade open-source relational database' },
    ports: [{ host: '5432', container: '5432' }],
    volumes: [{ host: 'postgres-data', container: '/var/lib/postgresql/data' }],
    envs: [
      { key: 'POSTGRES_PASSWORD', value: PASSWORD_TOKEN },
      { key: 'TZ', value: 'Asia/Shanghai' },
    ],
    note: {
      zh: 'postgres 用户密码已随机生成，请在创建前复制保存',
      en: 'A random postgres user password is generated — copy it before creating',
    },
  },
  {
    id: 'mongo',
    name: 'MongoDB',
    category: 'database',
    repository_name: 'library/mongo',
    registry: 'docker.io',
    tag: '7',
    desc: { zh: '面向文档的 NoSQL 数据库', en: 'Document-oriented NoSQL database' },
    ports: [{ host: '27017', container: '27017' }],
    volumes: [{ host: 'mongo-data', container: '/data/db' }],
    envs: [
      { key: 'MONGO_INITDB_ROOT_USERNAME', value: 'admin' },
      { key: 'MONGO_INITDB_ROOT_PASSWORD', value: PASSWORD_TOKEN },
    ],
    note: {
      zh: 'admin 用户密码已随机生成，请在创建前复制保存',
      en: 'A random admin password is generated — copy it before creating',
    },
  },
  {
    id: 'rabbitmq',
    name: 'RabbitMQ',
    category: 'mq',
    repository_name: 'library/rabbitmq',
    registry: 'docker.io',
    tag: '3-management',
    desc: { zh: '可靠的开源消息队列，含管理界面', en: 'Reliable open-source message broker with management UI' },
    ports: [
      { host: '5672', container: '5672' },
      { host: '15672', container: '15672' },
    ],
    volumes: [{ host: 'rabbitmq-data', container: '/var/lib/rabbitmq' }],
    envs: [
      { key: 'RABBITMQ_DEFAULT_USER', value: 'admin' },
      { key: 'RABBITMQ_DEFAULT_PASS', value: PASSWORD_TOKEN },
    ],
    note: {
      zh: '管理界面在 15672 端口；admin 密码已随机生成，请在创建前复制保存',
      en: 'Management UI on port 15672; a random admin password is generated — copy it before creating',
    },
  },
  {
    id: 'nginx',
    name: 'Nginx',
    category: 'web',
    repository_name: 'library/nginx',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '高性能 Web 服务器与反向代理', en: 'High-performance web server and reverse proxy' },
    ports: [{ host: '80', container: '80' }],
    volumes: [{ host: 'nginx-html', container: '/usr/share/nginx/html' }],
    envs: [],
  },
  {
    id: 'adminer',
    name: 'Adminer',
    category: 'web',
    repository_name: 'library/adminer',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '轻量数据库管理界面，支持 MySQL/PostgreSQL 等', en: 'Lightweight database management UI for MySQL, PostgreSQL and more' },
    ports: [{ host: '8081', container: '8080' }],
    volumes: [],
    envs: [],
  },
  {
    id: 'phpmyadmin',
    name: 'phpMyAdmin',
    category: 'web',
    repository_name: 'library/phpmyadmin',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '经典 MySQL/MariaDB Web 管理工具', en: 'Classic web administration tool for MySQL/MariaDB' },
    ports: [{ host: '8082', container: '80' }],
    volumes: [],
    envs: [{ key: 'PMA_ARBITRARY', value: '1' }],
    note: {
      zh: '登录页输入数据库主机与凭据即可连接任意 MySQL/MariaDB',
      en: 'Enter the database host and credentials on the login page',
    },
  },
  {
    id: 'whoami',
    name: 'Whoami',
    category: 'web',
    repository_name: 'traefik/whoami',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '打印请求信息的微型 HTTP 测试服务', en: 'Tiny HTTP test service printing request info' },
    ports: [{ host: '8090', container: '80' }],
    volumes: [],
    envs: [],
  },
  {
    id: 'gitea',
    name: 'Gitea',
    category: 'devops',
    repository_name: 'gitea/gitea',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '轻量自托管 Git 服务', en: 'Lightweight self-hosted Git service' },
    ports: [
      { host: '3000', container: '3000' },
      { host: '2222', container: '22' },
    ],
    volumes: [{ host: 'gitea-data', container: '/data' }],
    envs: [],
    note: {
      zh: '首次访问会进入安装向导，请及时完成初始化',
      en: 'The first visit runs the install wizard — finish it promptly',
    },
  },
  {
    id: 'grafana',
    name: 'Grafana',
    category: 'devops',
    repository_name: 'grafana/grafana',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '可视化监控与仪表板平台', en: 'Observability dashboards and visualization platform' },
    ports: [{ host: '3002', container: '3000' }],
    volumes: [{ host: 'grafana-data', container: '/var/lib/grafana' }],
    envs: [],
    note: {
      zh: '默认登录 admin/admin，首次登录会强制修改密码',
      en: 'Default login admin/admin — you are forced to change the password on first login',
    },
  },
  {
    id: 'uptime-kuma',
    name: 'Uptime Kuma',
    category: 'devops',
    repository_name: 'louislam/uptime-kuma',
    registry: 'docker.io',
    tag: '1',
    desc: { zh: '美观易用的自托管监控告警工具', en: 'Fancy self-hosted monitoring and alerting tool' },
    ports: [{ host: '3001', container: '3001' }],
    volumes: [{ host: 'uptime-kuma-data', container: '/app/data' }],
    envs: [],
    note: {
      zh: '首次访问创建管理员账号，请及时完成',
      en: 'The first visit creates the admin account — do it promptly',
    },
  },
  {
    id: 'nextcloud',
    name: 'Nextcloud',
    category: 'app',
    repository_name: 'library/nextcloud',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '自托管私有云盘与协作平台', en: 'Self-hosted private cloud storage and collaboration platform' },
    ports: [{ host: '8083', container: '80' }],
    volumes: [{ host: 'nextcloud-data', container: '/var/www/html' }],
    envs: [],
    note: {
      zh: '首次访问创建管理员账号',
      en: 'The first visit creates the admin account',
    },
  },
  {
    id: 'vaultwarden',
    name: 'Vaultwarden',
    category: 'app',
    repository_name: 'vaultwarden/server',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: 'Bitwarden 兼容的轻量密码管理服务端', en: 'Lightweight Bitwarden-compatible password manager server' },
    ports: [{ host: '8084', container: '80' }],
    volumes: [{ host: 'vaultwarden-data', container: '/data' }],
    envs: [],
    note: {
      zh: '浏览器端 Web 密码库需要 HTTPS（或 localhost）才能正常使用',
      en: 'The web vault needs HTTPS (or localhost) to work in browsers',
    },
  },
  {
    id: 'jellyfin',
    name: 'Jellyfin',
    category: 'app',
    repository_name: 'jellyfin/jellyfin',
    registry: 'docker.io',
    tag: 'latest',
    desc: { zh: '自由开源的媒体服务器', en: 'Free and open-source media server' },
    ports: [{ host: '8096', container: '8096' }],
    volumes: [
      { host: 'jellyfin-config', container: '/config' },
      { host: 'jellyfin-cache', container: '/cache' },
    ],
    envs: [],
    note: {
      zh: '创建后请把媒体目录以额外卷挂载进容器（如宿主机路径 → /media）再使用',
      en: 'After creation, add your media folders as extra volumes (e.g. host path → /media)',
    },
  },
]

// 仅字母数字，避免密码在 shell/YAML/连接串中需要转义
const PASSWORD_CHARSET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'

export function generatePassword(len = 16): string {
  const bytes = new Uint8Array(len)
  crypto.getRandomValues(bytes)
  let out = ''
  for (let i = 0; i < len; i++) {
    out += PASSWORD_CHARSET[bytes[i]! % PASSWORD_CHARSET.length]
  }
  return out
}

// 由目录条目生成建容器表单。返回深拷贝，调用方可以随意修改；
// 每个 __RANDOM_PASSWORD__ 占位符独立替换为新生成的随机密码。
export function buildDeployForm(
  app: CatalogApp,
  source: PullSource
): { form: ContainerCreateForm; generatedPassword: boolean } {
  let generatedPassword = false
  const envs = app.envs.map((env) => {
    if (env.value !== PASSWORD_TOKEN) return { ...env }
    generatedPassword = true
    return { key: env.key, value: generatePassword() }
  })
  const form: ContainerCreateForm = {
    image: buildImageRef(
      { repository_name: app.repository_name, registry: app.registry, pull_domain: MIRROR_DOMAIN },
      app.tag,
      source
    ),
    name: app.id,
    restart: 'unless-stopped',
    network: 'bridge',
    ports: app.ports.map((p) => ({ ...p })),
    volumes: app.volumes.map((v) => ({ ...v })),
    envs,
  }
  return { form, generatedPassword }
}
