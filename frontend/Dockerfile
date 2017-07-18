FROM binocarlos/yarn-base
MAINTAINER kaiyadavenport@gmail.com
WORKDIR /app
COPY ./package.json /app/package.json
COPY ./yarn.lock /app/yarn.lock
RUN yarn install
COPY ./ /app/frontend
ENTRYPOINT ["yarn", "run"]
CMD ["watch"]