import { describe, expect, it } from 'vitest'
import type { git } from '../../wailsjs/go/models'
import { pushWords } from './pushPlan'

const plan = (p: Partial<git.PushPlan>) => ({ branch: 'feature', remote: 'origin', ref: 'feature', setUpstream: false, why: 'its upstream', appDefaults: [], ask: false, remotes: ['origin', 'fork'], ...p }) as git.PushPlan

describe('pushWords', () => {
  it('says where an ordinary push goes', () => {
    expect(pushWords(plan({}))).toMatchObject({ label: 'Push', title: 'Push feature to origin/feature' })
  })
  it('names the remote when a setting sends it elsewhere', () => {
    expect(pushWords(plan({ remote: 'fork', why: 'remote.pushDefault' }))).toMatchObject({ label: 'Push → fork', title: 'Push feature to fork/feature (remote.pushDefault)' })
    expect(pushWords(plan({ remote: 'fork', why: 'branch.feature.pushRemote' })).label).toBe('Push → fork')
  })
  it('says a first push tracks what it makes', () => {
    expect(pushWords(plan({ setUpstream: true, why: 'origin' })).title).toBe('Push feature to origin/feature, its first push, and track it there')
  })
  it('asks, or says why not', () => {
    expect(pushWords(plan({ ask: true, remote: '', ref: '' })).label).toBe('Push…')
    expect(pushWords(plan({ refusal: 'push.default is nothing, so git pushes nothing unless told where.' })).title).toContain('push.default is nothing')
    expect(pushWords(null).label).toBe('Push')
  })
})
