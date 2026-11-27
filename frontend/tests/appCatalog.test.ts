import { describe, it, expect } from 'vitest'
import { CATALOG_APPS, buildDeployForm, generatePassword } from '@/data/appCatalog'
import { isValidRepoName, isValidTag } from '@/utils/imageRef'

const VALID_CATEGORIES = ['database', 'mq', 'web', 'devops', 'app']

describe('CATALOG_APPS data', () => {
  it('contains exactly 15 entries', () => {
    expect(CATALOG_APPS).toHaveLength(15)
  })

  it('has unique ids', () => {
    const ids = CATALOG_APPS.map((a) => a.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('uses valid repository names and tags', () => {
    for (const app of CATALOG_APPS) {
      expect(isValidRepoName(app.repository_name), app.repository_name).toBe(true)
      expect(isValidTag(app.tag), app.tag).toBe(true)
    }
  })

  it('uses valid categories', () => {
    for (const app of CATALOG_APPS) {
      expect(VALID_CATEGORIES).toContain(app.category)
    }
  })

  it('ports are numeric strings in 1-65535 and host ports are unique across entries', () => {
    const hostPorts = new Map<string, string>()
    for (const app of CATALOG_APPS) {
      for (const port of [...app.ports.map((p) => p.host), ...app.ports.map((p) => p.container)]) {
        expect(port).toMatch(/^[0-9]+$/)
        const n = Number(port)
        expect(n).toBeGreaterThanOrEqual(1)
        expect(n).toBeLessThanOrEqual(65535)
      }
      for (const p of app.ports) {
        const existing = hostPorts.get(p.host)
        expect(existing, `host port ${p.host} used by both ${existing} and ${app.id}`).toBeUndefined()
        hostPorts.set(p.host, app.id)
      }
    }
  })

  it('volumes use absolute container paths and valid named-volume hosts', () => {
    const NAMED_VOLUME_RE = /^[a-z0-9][a-z0-9_.-]*$/
    for (const app of CATALOG_APPS) {
      for (const v of app.volumes) {
        expect(v.container.startsWith('/'), `${app.id} ${v.container}`).toBe(true)
        expect(NAMED_VOLUME_RE.test(v.host), `${app.id} ${v.host}`).toBe(true)
      }
    }
  })

  it('env keys are valid environment variable names', () => {
    for (const app of CATALOG_APPS) {
      for (const env of app.envs) {
        expect(env.key).toMatch(/^[A-Z_][A-Z0-9_]*$/)
      }
    }
  })
})

describe('generatePassword', () => {
  it('generates a 16-char alphanumeric password by default', () => {
    const pw = generatePassword()
    expect(pw).toHaveLength(16)
    expect(pw).toMatch(/^[A-Za-z0-9]+$/)
  })

  it('two calls produce different passwords', () => {
    expect(generatePassword()).not.toBe(generatePassword())
  })

  it('respects a custom length', () => {
    expect(generatePassword(32)).toHaveLength(32)
    expect(generatePassword(1)).toHaveLength(1)
  })
})

describe('buildDeployForm', () => {
  const mysql = CATALOG_APPS.find((a) => a.id === 'mysql')!
  const vaultwarden = CATALOG_APPS.find((a) => a.id === 'vaultwarden')!
  const nginx = CATALOG_APPS.find((a) => a.id === 'nginx')!

  it('builds the mirror image ref', () => {
    const { form } = buildDeployForm(mysql, 'mirror')
    expect(form.image).toBe('docker.1ms.run/library/mysql:8')
  })

  it('builds the origin image ref stripping the library prefix', () => {
    const { form } = buildDeployForm(mysql, 'origin')
    expect(form.image).toBe('mysql:8')
  })

  it('keeps non-library namespaces on the origin ref', () => {
    const { form } = buildDeployForm(vaultwarden, 'origin')
    expect(form.image).toBe('vaultwarden/server:latest')
  })

  it('replaces every password token with a fresh random password', () => {
    const { form, generatedPassword } = buildDeployForm(mysql, 'mirror')
    expect(generatedPassword).toBe(true)
    for (const env of form.envs) {
      expect(env.value).not.toBe('__RANDOM_PASSWORD__')
    }
    const pw = form.envs.find((e) => e.key === 'MYSQL_ROOT_PASSWORD')!.value
    expect(pw).toMatch(/^[A-Za-z0-9]{16}$/)
  })

  it('reports generatedPassword false for apps without secrets', () => {
    const { generatedPassword } = buildDeployForm(nginx, 'mirror')
    expect(generatedPassword).toBe(false)
  })

  it('two calls produce different passwords', () => {
    const a = buildDeployForm(mysql, 'mirror').form.envs.find((e) => e.key === 'MYSQL_ROOT_PASSWORD')!.value
    const b = buildDeployForm(mysql, 'mirror').form.envs.find((e) => e.key === 'MYSQL_ROOT_PASSWORD')!.value
    expect(a).not.toBe(b)
  })

  it('fills name, restart and network defaults', () => {
    const { form } = buildDeployForm(mysql, 'mirror')
    expect(form.name).toBe('mysql')
    expect(form.restart).toBe('unless-stopped')
    expect(form.network).toBe('bridge')
  })

  it('returns a deep copy isolated from the catalog data', () => {
    const { form } = buildDeployForm(mysql, 'mirror')
    form.ports[0]!.host = '9999'
    form.volumes[0]!.host = 'changed'
    form.envs.push({ key: 'EXTRA', value: '1' })
    expect(mysql.ports[0]!.host).toBe('3306')
    expect(mysql.volumes[0]!.host).toBe('mysql-data')
    expect(mysql.envs.some((e) => e.key === 'EXTRA')).toBe(false)
  })
})
