import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, type PropType } from 'vue'
import { Dialog, DialogContent } from '@/components/ui/dialog'

const TestComp = defineComponent({
  components: { Dialog, DialogContent },
  props: {
    contentProps: { type: Object as PropType<Record<string, unknown>>, default: () => ({}) },
  },
  template: `
    <Dialog :open="true">
      <DialogContent v-bind="contentProps"><p>content</p></DialogContent>
    </Dialog>
  `,
})

describe('DialogContent close button', () => {
  it('renders the close button by default', async () => {
    const wrapper = mount(TestComp, { attachTo: document.body })
    await new Promise((r) => setTimeout(r, 50))
    const btn = document.body.querySelector('[data-slot="dialog-close"]')
    expect(btn).not.toBeNull()
    expect(btn?.querySelector('svg')).not.toBeNull()
    wrapper.unmount()
  })

  it('hides the close button when showCloseButton is false', async () => {
    const wrapper = mount(TestComp, {
      attachTo: document.body,
      props: { contentProps: { showCloseButton: false } },
    })
    await new Promise((r) => setTimeout(r, 50))
    expect(document.body.querySelector('[data-slot="dialog-close"]')).toBeNull()
    wrapper.unmount()
  })
})
