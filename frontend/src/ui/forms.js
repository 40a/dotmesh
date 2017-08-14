import formWrapper from 'template-ui/lib/plugins/form/wrapper'
import models from 'template-ui/lib/plugins/form/models'
import fields from 'template-ui/lib/plugins/form/fields'
import validators from 'template-ui/lib/plugins/form/validators'

const authLogin = formWrapper({
  name: 'authLogin',
  fields: {
    username: models.string({
      component: fields.input,
      validate: [validators.required]
    }),
    password: models.string({
      type: 'password',
      component: fields.input,
      validate: validators.required
    }),
    remember: models.raw({
      title: 'Remember Me?',
      component: fields.checkbox
    })
  }
})

const authRegister = formWrapper({
  name: 'authRegister',
  fields: {
    email: models.string({
      component: fields.input,
      validate: [validators.required,validators.email]
    }),
    username: models.string({
      component: fields.input,
      validate: [validators.required]
    }),
    password: models.string({
      type: 'password',
      component: fields.input,
      validate: validators.required
    }),
    remember: models.raw({
      title: 'Remember Me?',
      component: fields.checkbox
    })
  }
})

const forms = {
  authLogin,
  authRegister
}

export default forms