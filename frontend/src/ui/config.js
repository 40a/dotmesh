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
  search: 'search'
}

const config = {
  title:'Datamesh Console',
  basepath:'/ui',
  rpcNamespace: 'DatameshRPC',
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
      ['-'],
      ['/help', 'Help', icons.help],
      ['-'],
      ['authLogout', 'Logout', icons.logout]
    ]
  },
  icons
}

export default config