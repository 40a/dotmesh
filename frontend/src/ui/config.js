const icons = {
  dashboard: 'dashboard',
  help: 'help_outline',
  about: 'info_outline',
  home: 'home',
  menu: 'menu',
  options: 'more_vert',
  logout: 'exit_to_app',
  login: 'account_circle',
  register: 'create',
  cancel: 'clear',
  revert: 'undo',
  save: 'send',
  add: 'add',
  edit: 'create',
  delete: 'delete',
  folder_open: 'keyboard_arrow_right',
  view: 'visibility',
  actions: 'more_vert',
  folder: 'folder',
  folderadd: 'create_new_folder',
  settings: 'settings',
  search: 'search',
  disk: 'storage',
  server: 'computer',
  payment: 'credit_card',
  back: 'arrow_back',
  forward: 'arrow_forward'
}

const config = {
  title:'Datamesh Console',
  basepath:'/ui',
  rpcNamespace: 'DatameshRPC',
  devmodePlanName: 'developer',
  rpcUrl: '/rpc',
  userValueName: 'user',
  userLocalStorageName: 'user',
  initialState: {
    value: {
      config: {},
      initialized: false,
      user: null,
      menuOpen: false
    }
  },
  // milliseconds delay for each iteration of the control loop
  controlLoopInterval: process.env.NODE_ENV == 'development' ? 10000 : 1000,
  repolist: {
    pageSize: 3
  },
  images: {
    appbar: '/images/datamesh-on-dark.png'
  },
  menu: {
    guest: [
      ['/', 'Home', icons.dashboard],
      ['/login', 'Login', icons.login],
      ['/register', 'Register', icons.register],
      ['-'],
      ['/help', 'Help', icons.help]
    ],
    user: [
      ['/dashboard', 'Dashboard', icons.dashboard],
      ['/repos', 'Repos', icons.disk],
      ['/servers', 'Servers', icons.server],
      ['/payment', 'Payment', icons.payment],
      ['-'],
      ['/help', 'Help', icons.help],
      ['-'],
      ['authLogout', 'Logout', icons.logout]
    ]
  },
  icons
}

if(process.env.NODE_ENV == 'development') {
  console.log('-------------------------------------------');
  console.log('-------------------------------------------');
  console.log('-------------------------------------------');
  console.log('-------------------------------------------');
  console.log('-------------------------------------------');
  console.log('DEV MODE')
  console.log('data loop is 10 seconds not 1')
}

export default config
