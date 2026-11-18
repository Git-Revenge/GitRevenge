import { mount } from 'svelte'
import './style.css'
import App from './App.svelte'
import { suppressNativeMenu } from './lib/nativeMenu'
import { guardLinks } from './lib/linkGuard'
import { BrowserOpenURL } from '../wailsjs/runtime/runtime.js'

suppressNativeMenu(window)
guardLinks(window, BrowserOpenURL)

const app = mount(App, { target: document.getElementById('app')! })

export default app
