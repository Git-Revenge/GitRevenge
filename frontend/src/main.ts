import { mount } from 'svelte'
import './style.css'
import App from './App.svelte'
import { suppressNativeMenu } from './lib/nativeMenu'
import { guardLinks } from './lib/linkGuard'
import { BrowserOpenURL, OnFileDrop } from '../wailsjs/runtime/runtime.js'

suppressNativeMenu(window)
guardLinks(window, BrowserOpenURL)
// Files dropped on the window (#1025). Wails listens for the drop once this is called and gives
// the paths to the backend, which marks them readable and sends "patches:dropped".
OnFileDrop(() => {}, false)

const app = mount(App, { target: document.getElementById('app')! })

export default app
