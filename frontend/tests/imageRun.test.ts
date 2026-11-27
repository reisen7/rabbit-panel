import { describe, it, expect } from 'vitest'
import { runnableImageRef } from '@/utils/imageRun'

describe('runnableImageRef', () => {
  it('joins name and tag for normal images', () => {
    expect(runnableImageRef({ id: 'abc123', name: 'nginx', tag: 'alpine' })).toBe('nginx:alpine')
  })

  it('handles registry names containing a port', () => {
    expect(runnableImageRef({ id: 'abc123', name: 'localhost:5000/foo/bar', tag: '1.0' })).toBe(
      'localhost:5000/foo/bar:1.0'
    )
  })

  it('falls back to id for dangling <none> images', () => {
    expect(runnableImageRef({ id: 'abc123', name: '<none>', tag: '<none>' })).toBe('abc123')
  })

  it('falls back to id when name is empty', () => {
    expect(runnableImageRef({ id: 'abc123', name: '', tag: 'latest' })).toBe('abc123')
  })

  it('falls back to id when tag is empty', () => {
    expect(runnableImageRef({ id: 'abc123', name: 'nginx', tag: '' })).toBe('abc123')
  })
})
