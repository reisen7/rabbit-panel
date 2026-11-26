import { describe, it, expect } from 'vitest'
import {
  buildImageRef,
  buildPullCommand,
  isValidHost,
  isValidRepoName,
  isValidTag,
  resolveTag,
  sanitizeSearchItem,
} from '@/utils/imageRef'

const rawItem = (over: Record<string, unknown> = {}) => ({
  id: 532,
  name: 'php',
  namespace: 'library',
  registry: 'docker.io',
  repository_name: 'library/php',
  pull_domain: 'docker.1ms.run',
  logo_url: 'https://static.1ms.run/image/avatar/abc',
  description: 'PHP image',
  pull_count: 1417356357,
  star_count: 7890,
  is_official: true,
  ...over,
})

describe('isValidRepoName', () => {
  it('accepts valid names', () => {
    for (const name of ['php', 'library/php', 'my-org/img.name_tag-1', 'a/b/c/d']) {
      expect(isValidRepoName(name)).toBe(true)
    }
  })

  it('rejects invalid and injection names', () => {
    for (const name of [
      'PHP',
      'php; rm -rf /',
      '$(id)',
      'a b',
      '../x',
      '`id`',
      '',
      'a//b',
      '-php',
      'php:latest',
      'x'.repeat(201),
    ]) {
      expect(isValidRepoName(name)).toBe(false)
    }
  })
})

describe('isValidTag', () => {
  it('accepts valid tags', () => {
    for (const tag of ['latest', '8.4-cli', 'v1_2.3', 'A.B-C_D']) {
      expect(isValidTag(tag)).toBe(true)
    }
  })

  it('rejects invalid tags', () => {
    for (const tag of ['a b', 'x\ny', '-bad', '.bad', '', 'x'.repeat(129), '`id`']) {
      expect(isValidTag(tag)).toBe(false)
    }
  })
})

describe('isValidHost', () => {
  it('accepts empty and valid hosts', () => {
    for (const host of ['', 'docker.1ms.run', 'reg:5000', 'localhost', 'a.b.c.d:443']) {
      expect(isValidHost(host)).toBe(true)
    }
  })

  it('rejects invalid hosts', () => {
    for (const host of ['ev il.com', 'a;id', 'x\ny', 'host:abc', 'host:', '.bad.com']) {
      expect(isValidHost(host)).toBe(false)
    }
  })
})

describe('sanitizeSearchItem', () => {
  it('passes through a valid item', () => {
    const item = sanitizeSearchItem(rawItem())
    expect(item).not.toBeNull()
    expect(item?.repository_name).toBe('library/php')
    expect(item?.pull_domain).toBe('docker.1ms.run')
    expect(item?.is_official).toBe(true)
    expect(item?.pull_count).toBe(1417356357)
  })

  it('drops items with invalid repository_name', () => {
    expect(sanitizeSearchItem(rawItem({ repository_name: 'php; rm -rf /' }))).toBeNull()
    expect(sanitizeSearchItem(rawItem({ repository_name: '$(id)' }))).toBeNull()
    expect(sanitizeSearchItem(rawItem({ repository_name: 'a b' }))).toBeNull()
    expect(sanitizeSearchItem(rawItem({ repository_name: 'PHP' }))).toBeNull()
    expect(sanitizeSearchItem(rawItem({ repository_name: 'a`id`b' }))).toBeNull()
  })

  it('drops items with invalid pull_domain or registry', () => {
    expect(sanitizeSearchItem(rawItem({ pull_domain: 'evil;id' }))).toBeNull()
    expect(sanitizeSearchItem(rawItem({ pull_domain: 'ev il.com' }))).toBeNull()
    expect(sanitizeSearchItem(rawItem({ pull_domain: 'evil.com\n' }))).toBeNull()
    expect(sanitizeSearchItem(rawItem({ registry: 'reg;rm' }))).toBeNull()
  })

  it('silently clears invalid logo_url', () => {
    expect(sanitizeSearchItem(rawItem({ logo_url: 'javascript:alert(1)' }))?.logo_url).toBe('')
    expect(sanitizeSearchItem(rawItem({ logo_url: 'ftp://x' }))?.logo_url).toBe('')
    expect(sanitizeSearchItem(rawItem({ logo_url: 'https://a b' }))?.logo_url).toBe('')
    expect(sanitizeSearchItem(rawItem({ logo_url: 'http://ok.cdn/x.png' }))?.logo_url).toBe(
      'http://ok.cdn/x.png'
    )
  })

  it('drops non-object and malformed input', () => {
    expect(sanitizeSearchItem(null)).toBeNull()
    expect(sanitizeSearchItem('string')).toBeNull()
    expect(sanitizeSearchItem(42)).toBeNull()
    expect(sanitizeSearchItem({})).toBeNull()
  })

  it('clamps counts to non-negative integers', () => {
    const item = sanitizeSearchItem(rawItem({ pull_count: -5, star_count: 3.9 }))
    expect(item?.pull_count).toBe(0)
    expect(item?.star_count).toBe(3)
  })
})

describe('buildImageRef / buildPullCommand', () => {
  const item = {
    repository_name: 'library/php',
    pull_domain: 'docker.1ms.run',
    registry: 'docker.io',
  }

  it('builds mirror ref with pull_domain', () => {
    expect(buildImageRef(item, 'latest', 'mirror')).toBe('docker.1ms.run/library/php:latest')
    expect(buildImageRef(item, '8.4', 'mirror')).toBe('docker.1ms.run/library/php:8.4')
  })

  it('falls back to origin ref when pull_domain empty', () => {
    const noMirror = { ...item, pull_domain: '' }
    expect(buildImageRef(noMirror, 'latest', 'mirror')).toBe('php:latest')
  })

  it('strips library/ prefix for docker.io origin', () => {
    expect(buildImageRef(item, 'latest', 'origin')).toBe('php:latest')
    const emptyReg = { ...item, registry: '' }
    expect(buildImageRef(emptyReg, 'latest', 'origin')).toBe('php:latest')
  })

  it('keeps non-docker.io registry in origin ref', () => {
    const gcr = { repository_name: 'proj/img', pull_domain: '', registry: 'gcr.io' }
    expect(buildImageRef(gcr, 'v1', 'origin')).toBe('gcr.io/proj/img:v1')
  })

  it('falls back to latest for invalid or empty tag', () => {
    expect(buildImageRef(item, '', 'mirror')).toBe('docker.1ms.run/library/php:latest')
    expect(buildImageRef(item, 'a b', 'mirror')).toBe('docker.1ms.run/library/php:latest')
    expect(resolveTag('x\ny')).toBe('latest')
    expect(resolveTag('8.4')).toBe('8.4')
  })

  it('builds the full pull command', () => {
    expect(buildPullCommand(item, '8.4', 'mirror')).toBe('docker pull docker.1ms.run/library/php:8.4')
    expect(buildPullCommand(item, 'latest', 'origin')).toBe('docker pull php:latest')
  })

  it('injection strings never reach the command', () => {
    expect(buildPullCommand(item, 'x; rm -rf /', 'mirror')).toBe(
      'docker pull docker.1ms.run/library/php:latest'
    )
    expect(buildPullCommand(item, '$(id)', 'origin')).toBe('docker pull php:latest')
  })
})
