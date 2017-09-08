import formWrapper from 'template-ui/lib/plugins/form/wrapper'
import models from 'template-ui/lib/plugins/form/models'
import fields from 'template-ui/lib/plugins/form/fields'
import validators from 'template-ui/lib/plugins/form/validators'

const authLogin = formWrapper({
  name: 'authLogin',
  fields: {
    Name: models.string({
      component: fields.input,
      validate: [validators.required]
    }),
    Password: models.string({
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
      component: fields.input,
      validate: [validators.required,validators.email]
    }),
    Name: models.string({
      component: fields.input,
      validate: [validators.required]
    }),
    Password: models.string({
      type: 'password',
      component: fields.input,
      validate: validators.required
    })
  }
})

const forms = {
  authLogin,
  authRegister
}

export default forms