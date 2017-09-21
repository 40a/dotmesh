import formWrapper from 'template-ui/lib/plugins/form/wrapper'
import models from 'template-ui/lib/plugins/form/models'
import fields from 'template-ui/lib/plugins/form/fields'
import validators from 'template-ui/lib/plugins/form/validators'

const authLogin = formWrapper({
  name: 'authLogin',
  fields: {
    Name: models.string({
      title: 'Username',
      component: fields.input,
      validate: [validators.required]
    }),
    Password: models.string({
      title: 'Password',
      type: 'password',
      component: fields.input,
      validate: validators.required
    })
  }
})

const authRegister = formWrapper({
  name: 'authRegister',
  fields: {
    Email: models.string({
      title: 'Email',
      component: fields.input,
      validate: [validators.required,validators.email]
    }),
    Name: models.string({
      title: 'Username',
      component: fields.input,
      validate: [validators.required]
    }),
    Password: models.string({
      title: 'Password',
      type: 'password',
      component: fields.input,
      validate: validators.required
    })
  }
})

const repo = formWrapper({
  name: 'repo',
  fields: {
    Name: models.string({
      title: 'Name',
      component: fields.input,
      validate: [validators.required]
    })
  }
})

const forms = {
  authLogin,
  authRegister,
  repo
}

export default forms